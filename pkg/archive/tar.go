// Package archive contains functionality for archiving/unarchiving streams.
package archive

import (
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/moby/go-archive"
	"github.com/moby/go-archive/compression"
)

// Compression is the state represents if compressed or not.
type Compression compression.Compression

// Compressions types.
const (
	Uncompressed = Compression(archive.Uncompressed) // Uncompressed represents the uncompressed.
	Bzip2        = Compression(archive.Bzip2)        // Bzip2 is bzip2 compression algorithm.
	Gzip         = Compression(archive.Gzip)         // Gzip is gzip compression algorithm.
	Xz           = Compression(archive.Xz)           // Xz is xz compression algorithm.
	Zstd         = Compression(archive.Zstd)         // Zstd is zstd compression algorithm.
)

// TarOptions wraps the tar options.
type TarOptions struct {
	IncludeFiles    []string
	ExcludePatterns []string
	Compression     Compression
	RebaseNames     map[string]string
	SrcInfo         CopyInfo
}

// CopyInfo holds basic info about the source
// or destination path of a copy operation.
type CopyInfo archive.CopyInfo

type stackReadCloser struct {
	s []io.ReadCloser
}

func (s *stackReadCloser) Push(r io.ReadCloser)             { s.s = append(s.s, r) }
func (s *stackReadCloser) Read(p []byte) (n int, err error) { return s.s[len(s.s)-1].Read(p) }
func (s *stackReadCloser) Close() error {
	errs := make([]error, 0, len(s.s))
	for i := len(s.s) - 1; i >= 0; i-- {
		errs = append(errs, s.s[i].Close())
	}
	return errors.Join(errs...)
}

// Tar creates an archive from the source.
func Tar(src CopyInfo, dst CopyInfo, opts *TarOptions) (io.ReadCloser, error) {
	var err error
	if opts == nil {
		opts = &TarOptions{}
	}
	stack := &stackReadCloser{s: make([]io.ReadCloser, 0)}

	srcInfo := archive.CopyInfo(src)
	if _, err = os.Lstat(srcInfo.Path); err != nil {
		return nil, err
	}

	// Tar resource with rebasing name.
	sourceDir, sourceBase := archive.SplitPathDirEntry(srcInfo.Path)
	tarOpts := archive.TarResourceRebaseOpts(sourceBase, srcInfo.RebaseName)
	tarOpts.ExcludePatterns = opts.ExcludePatterns
	maps.Insert(tarOpts.RebaseNames, maps.All(opts.RebaseNames))
	tarOpts.Compression = compression.Compression(opts.Compression)
	slices.AppendSeq(tarOpts.IncludeFiles, slices.Values(opts.IncludeFiles))

	r, err := archive.TarWithOptions(sourceDir, tarOpts)
	if err != nil {
		return r, err
	}
	stack.Push(r)

	// Update destination names.
	dstInfo := archive.CopyInfo(dst)
	_, rprep, err := archive.PrepareArchiveCopy(r, srcInfo, dstInfo)
	if err != nil {
		_ = stack.Close()
		return nil, err
	}
	stack.Push(rprep)

	return stack, nil
}

// CopyInfoSourcePath stats the given path to create a CopyInfo
// struct representing that resource for the source of an archive copy
// operation. The given path should be an absolute local path.
func CopyInfoSourcePath(path string, followLink bool) (CopyInfo, error) {
	res, err := archive.CopyInfoSourcePath(path, followLink)
	return CopyInfo(res), err
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive may be compressed with one of the following algorithms:
// identity (uncompressed), gzip, bzip2, xz.
func Untar(content io.ReadCloser, dstPath string, opts *TarOptions) error {
	preArchive := content
	if opts == nil {
		opts = &TarOptions{}
	}
	srcInfo := archive.CopyInfo(opts.SrcInfo)
	if len(srcInfo.RebaseName) != 0 {
		srcBase := filepath.Base(srcInfo.Path)
		preArchive = archive.RebaseArchiveEntries(content, srcBase, srcInfo.RebaseName)
		defer preArchive.Close()
	}

	dstInfo, err := archive.CopyInfoDestinationPath(dstPath)
	if err != nil {
		return err
	}

	dstDir, copyArchive, err := archive.PrepareArchiveCopy(preArchive, srcInfo, dstInfo)
	if err != nil {
		return err
	}
	defer copyArchive.Close()

	options := &archive.TarOptions{
		NoLchown:             true,
		NoOverwriteDirNonDir: true,
	}

	return archive.Untar(copyArchive, dstDir, options)

}
