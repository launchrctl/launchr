package embed

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing/fstest"
	"time"

	"github.com/launchrctl/launchr/pkg/action"
)

func createActionTar(cfs fs.FS, buildPath string) (string, []action.Action, error) {
	name := "actions.tar.gz"
	target := filepath.Join(buildPath, name)
	// Discover actions.
	ad := action.NewYamlDiscovery(action.NewDiscoveryFS(cfs, ""))
	actions, err := ad.Discover()
	if err != nil {
		return name, nil, err
	}
	// Create tar file with actions.
	f, err := os.Create(filepath.Clean(target))
	if err != nil {
		return name, nil, err
	}
	defer func() {
		_ = f.Close()
	}()
	// Pack actions in a file.
	err = TarGzEmbedActions(f, actions)

	return name, actions, err
}

// TarGzEmbedActions tars and gzip action files to a file f.
func TarGzEmbedActions(f io.Writer, actions []action.Action) error {
	gzw := gzip.NewWriter(f)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()
	now := time.Now()

	for _, a := range actions {
		act, ok := a.(*action.ContainerAction)
		if !ok {
			continue
		}
		c, err := act.DefinitionEncoded()
		if err != nil {
			return err
		}

		h := &tar.Header{
			Name:    act.Filepath(),
			Mode:    0600,
			ModTime: now,
			Size:    int64(len(c)),
		}

		if err := tw.WriteHeader(h); err != nil {
			return err
		}

		if _, err := tw.Write(c); err != nil {
			return err
		}
	}

	return nil
}

// UntarFsBytes unzip and untar bytes to in-memory FS.
func UntarFsBytes(t []byte) (fs.FS, error) {
	r := bytes.NewBuffer(t)
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	mfs := fstest.MapFS{}

	for {
		h, err := tr.Next()
		switch {
		// if no more files are found return
		case err == io.EOF:
			return mfs, nil
		// return any other error
		case err != nil:
			return nil, err
		// if the header is nil, just skip it (not sure how this happens)
		case h == nil:
			continue
		}

		// check the file type
		switch h.Typeflag {
		case tar.TypeDir:
			// if it's a dir, we don't care in mapfs.
			continue
		case tar.TypeReg:
			// if it's a file create it
			b := make([]byte, 0, 128)
			content := bytes.NewBuffer(b)

			// unzip content
			for {
				_, err := io.CopyN(content, tr, 1024)
				if err != nil {
					if err == io.EOF {
						break
					}
					return nil, err
				}
			}

			mfs[h.Name] = &fstest.MapFile{
				Data: content.Bytes(),
			}
		}
	}
}
