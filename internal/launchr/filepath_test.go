package launchr

import (
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMkdirTemp(t *testing.T) {
	t.Parallel()
	dir, err := MkdirTemp("test")
	require.NoError(t, err)
	require.NotEmpty(t, dir)
	stat, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, stat.IsDir())
	err = Cleanup()
	require.NoError(t, err)
	_, err = os.Stat(dir)
	require.True(t, os.IsNotExist(err))
}

func TestFsRealpath(t *testing.T) {
	t.Parallel()
	// Test basic dir fs.
	rootfs := os.DirFS("../../")
	path := FsRealpath(rootfs)
	assert.Equal(t, MustAbs("../../"), path)

	// Test subdir of fs.
	subfs, err := fs.Sub(rootfs, "internal")
	require.NoError(t, err)
	path = FsRealpath(subfs)
	assert.Equal(t, MustAbs("../"), path)

	// Test memory fs.
	memfs := (fsmy{
		"some/path/inside": "",
	}).MapFS()
	path = FsRealpath(memfs)
	assert.Equal(t, "", path)

	// Test subdir of memory fs.
	subfs, err = fs.Sub(memfs, "some")
	require.NoError(t, err)
	path = FsRealpath(subfs)
	assert.Equal(t, "", path)
}
