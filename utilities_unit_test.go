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

// TestRound100 confirms that round100 rounds up to the nearest multiple of 100.
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

// TestFirst confirms that first returns its first non-zero argument, or the zero value if none.
func TestFirst(t *testing.T) {
	// first returns the first non-zero value, or the zero value if none.
	require.Equal(t, 5, first(0, 5, 3))
	require.Equal(t, 3, first(3, 5))
	require.Equal(t, 0, first(0, 0))
	require.Equal(t, 0, first[int]())

	require.Equal(t, "a", first("", "a", "b"))
	require.Equal(t, "", first("", ""))
}

// TestIsFFmpegMediaType confirms that only the video, image, and audio categories need ffmpeg.
func TestIsFFmpegMediaType(t *testing.T) {
	require.True(t, isFFmpegMediaType("video"))
	require.True(t, isFFmpegMediaType("image"))
	require.True(t, isFFmpegMediaType("audio"))

	require.False(t, isFFmpegMediaType("text"))
	require.False(t, isFFmpegMediaType("application"))
	require.False(t, isFFmpegMediaType(""))
}

// TestGetTempFilename confirms that getTempFilename creates an empty, uniquely named file with the
// given extension in the OS temp directory.
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

// TestWriteTempFile confirms that writeTempFile copies a reader into a temp file with the given
// extension.
func TestWriteTempFile(t *testing.T) {
	name, err := writeTempFile(strings.NewReader("hello, world"), ".txt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(name) })

	require.True(t, strings.HasSuffix(name, ".txt"))

	contents, err := os.ReadFile(name)
	require.NoError(t, err)
	require.Equal(t, "hello, world", string(contents))
}

// TestWriteTempFile_ReadError confirms that writeTempFile fails when its reader returns an error.
func TestWriteTempFile_ReadError(t *testing.T) {
	// A reader that always errors causes the copy (and the function) to fail.
	_, err := writeTempFile(&errorReader{}, ".txt")
	require.Error(t, err)
}

// TestEnsureAferoFolderExists confirms that ensureAferoFolderExists creates a missing folder and
// accepts one that already exists.
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

// TestEnsureAferoFolderExists_MkdirError confirms that ensureAferoFolderExists fails on a read-only
// filesystem.
func TestEnsureAferoFolderExists_MkdirError(t *testing.T) {
	// A read-only filesystem cannot create the folder.
	readOnly := afero.NewReadOnlyFs(afero.NewMemMapFs())
	require.Error(t, ensureAferoFolderExists(readOnly, "uploads"))
}

// TestGetCoverPhoto_FFmpegNotInstalled confirms that getCoverPhoto fails when ffmpeg is not
// installed.
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

// Read always returns io.ErrUnexpectedEOF.
func (errorReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
