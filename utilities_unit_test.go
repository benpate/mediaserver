package mediaserver

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestRound100(t *testing.T) {
	run := func(input int, expected int) {
		require.Equal(t, expected, round100(input), "input=%d", input)
	}

	run(0, 0)
	run(1, 100)
	run(99, 100)
	run(100, 100)
	run(101, 200)
	run(250, 300)
	run(300, 300)
}

func TestFirst(t *testing.T) {
	// first returns the first non-zero value, or the zero value if none.
	require.Equal(t, 5, first(0, 5, 3))
	require.Equal(t, 3, first(3, 5))
	require.Equal(t, 0, first(0, 0))
	require.Equal(t, 0, first[int]())

	require.Equal(t, "a", first("", "a", "b"))
	require.Equal(t, "", first("", ""))
}

func TestIsFFmpegMediaType(t *testing.T) {
	require.True(t, isFFmpegMediaType("video"))
	require.True(t, isFFmpegMediaType("image"))
	require.True(t, isFFmpegMediaType("audio"))

	require.False(t, isFFmpegMediaType("text"))
	require.False(t, isFFmpegMediaType("application"))
	require.False(t, isFFmpegMediaType(""))
}

func TestGetTempFilename(t *testing.T) {
	name, err := getTempFilename(".jpg")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(name) })

	require.Equal(t, filepath.Clean(os.TempDir()), filepath.Dir(name))
	require.True(t, strings.HasSuffix(name, ".jpg"))
	require.Contains(t, name, "mediaserver-")

	// getTempFilename atomically creates the (empty) file...
	info, err := os.Stat(name)
	require.NoError(t, err)
	require.Equal(t, int64(0), info.Size())

	// ...and successive calls return distinct names.
	other, err := getTempFilename(".jpg")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(other) })
	require.NotEqual(t, name, other)
}

func TestWriteTempFile(t *testing.T) {
	name, err := writeTempFile(strings.NewReader("hello, world"), ".txt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(name) })

	require.True(t, strings.HasSuffix(name, ".txt"))

	contents, err := os.ReadFile(name)
	require.NoError(t, err)
	require.Equal(t, "hello, world", string(contents))
}

func TestWriteTempFile_ReadError(t *testing.T) {
	// A reader that always errors causes the copy (and the function) to fail.
	_, err := writeTempFile(&errorReader{}, ".txt")
	require.Error(t, err)
}

func TestEnsureAferoFolderExists(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Creating a new folder succeeds...
	require.NoError(t, ensureAferoFolderExists(fs, "uploads"))
	exists, err := afero.DirExists(fs, "uploads")
	require.NoError(t, err)
	require.True(t, exists)

	// ...and calling again on an existing folder is a no-op (no error).
	require.NoError(t, ensureAferoFolderExists(fs, "uploads"))
}

func TestEnsureAferoFolderExists_MkdirError(t *testing.T) {
	// A read-only filesystem cannot create the folder.
	readOnly := afero.NewReadOnlyFs(afero.NewMemMapFs())
	require.Error(t, ensureAferoFolderExists(readOnly, "uploads"))
}

func TestGetCoverPhoto_FFmpegNotInstalled(t *testing.T) {
	// Force the "not installed" branch deterministically, without ffmpeg.
	original := ffmpegInstalled
	ffmpegInstalled = func() bool { return false }
	t.Cleanup(func() { ffmpegInstalled = original })

	ms := newTestServer(t, nil)
	_, err := ms.getCoverPhoto(context.Background(), "http://example.com/cover.jpg")
	require.Error(t, err)
}

// errorReader is an io.Reader that always returns an error.
type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
