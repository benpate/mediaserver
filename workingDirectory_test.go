package mediaserver

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewWorkingDirectory_DefaultFolder(t *testing.T) {
	// An empty folder argument defaults to the OS temp directory.
	wd := NewWorkingDirectory("", time.Minute, 10)
	t.Cleanup(wd.Close)

	require.Equal(t, os.TempDir(), wd.folder)
}

func TestNewWorkingDirectory_CustomFolder(t *testing.T) {
	dir := t.TempDir()
	wd := NewWorkingDirectory(dir, time.Minute, 10)
	t.Cleanup(wd.Close)

	require.Equal(t, dir, wd.folder)
}

func TestWorkingDirectory_Filename(t *testing.T) {
	wd := NewWorkingDirectory("/base/dir", time.Minute, 10)
	t.Cleanup(wd.Close)

	require.Equal(t, filepath.Join("/base/dir", "file.txt"), wd.filename("file.txt"))
}

func TestWorkingDirectory_WriteExistsOpen(t *testing.T) {
	wd := NewWorkingDirectory(t.TempDir(), time.Minute, 10)
	t.Cleanup(wd.Close)

	require.False(t, wd.Exists("a.txt"))

	require.NoError(t, wd.Write("a.txt", strings.NewReader("AAA")))
	require.True(t, wd.Exists("a.txt"))

	file, err := wd.Open("a.txt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	contents, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "AAA", string(contents))
}

func TestWorkingDirectory_OpenMissing(t *testing.T) {
	wd := NewWorkingDirectory(t.TempDir(), time.Minute, 10)
	t.Cleanup(wd.Close)

	_, err := wd.Open("missing.txt")
	require.Error(t, err)
}

func TestWorkingDirectory_WriteError(t *testing.T) {
	// Writing into a folder that does not exist fails at os.Create.
	wd := NewWorkingDirectory(filepath.Join(t.TempDir(), "does", "not", "exist"), time.Minute, 10)
	t.Cleanup(wd.Close)

	err := wd.Write("a.txt", strings.NewReader("AAA"))
	require.Error(t, err)
}

func TestWorkingDirectory_WriteCopyError(t *testing.T) {
	// The file is created, but copying from a failing reader errors out.
	wd := NewWorkingDirectory(t.TempDir(), time.Minute, 10)
	t.Cleanup(wd.Close)

	require.Error(t, wd.Write("a.txt", errorReader{}))
}

func TestWorkingDirectory_WriteReplaceTriggersListener(t *testing.T) {
	// Writing the same name twice replaces the cache entry, which exercises the
	// deletion listener's "Replaced" branch (it must not delete the file).
	wd := NewWorkingDirectory(t.TempDir(), time.Minute, 10)
	t.Cleanup(wd.Close)

	require.NoError(t, wd.Write("a.txt", strings.NewReader("AAA")))
	require.NoError(t, wd.Write("a.txt", strings.NewReader("BBB")))

	// Give the listener a moment to run, then confirm the file still exists.
	time.Sleep(50 * time.Millisecond)
	require.True(t, wd.Exists("a.txt"))
}

// TestWorkingDirectory_Remove confirms that Remove deletes the file from disk
// (via the cache's deletion listener, which fires asynchronously).
func TestWorkingDirectory_Remove(t *testing.T) {
	wd := NewWorkingDirectory(t.TempDir(), time.Minute, 10)
	t.Cleanup(wd.Close)

	require.NoError(t, wd.Write("a.txt", strings.NewReader("AAA")))
	require.True(t, wd.Exists("a.txt"))

	wd.Remove("a.txt")

	require.Eventually(t, func() bool {
		return !wd.Exists("a.txt")
	}, time.Second, 10*time.Millisecond, "Remove should delete the file from disk")
}

// TestWorkingDirectory_RemoveAllAndClose confirms RemoveAll and Close run
// without panicking.
func TestWorkingDirectory_RemoveAllAndClose(t *testing.T) {
	wd := NewWorkingDirectory(t.TempDir(), time.Minute, 10)
	t.Cleanup(wd.Close)

	require.NoError(t, wd.Write("a.txt", strings.NewReader("AAA")))
	require.NoError(t, wd.Write("b.txt", strings.NewReader("BBB")))

	require.NotPanics(t, wd.RemoveAll)
}
