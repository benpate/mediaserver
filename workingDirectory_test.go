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

	filename, err := wd.filename("file.txt")
	require.NoError(t, err)
	require.Equal(t, filepath.Join("/base/dir", "file.txt"), filename)
}

// TestWorkingDirectory_FilenameEscape verifies that a name which would resolve
// outside the working folder is rejected instead of being silently cleaned by
// filepath.Join into a path somewhere else on the filesystem.
func TestWorkingDirectory_FilenameEscape(t *testing.T) {
	wd := NewWorkingDirectory("/base/dir", time.Minute, 10)
	t.Cleanup(wd.Close)

	for _, name := range []string{"../escaped.txt", "../../escaped.txt", "/etc/passwd", ""} {
		filename, err := wd.filename(name)
		require.Error(t, err, "name %q must be rejected", name)
		require.Empty(t, filename)
	}
}

// TestWorkingDirectory_WriteEscape verifies that the containment check is actually
// enforced on the public entry points, not just in the private helper.
func TestWorkingDirectory_WriteEscape(t *testing.T) {

	base := t.TempDir()
	inner := filepath.Join(base, "work")
	require.NoError(t, os.Mkdir(inner, 0700))

	wd := NewWorkingDirectory(inner, time.Minute, 10)
	t.Cleanup(wd.Close)

	escape := filepath.Join("..", "escaped.txt")

	require.Error(t, wd.Write(escape, strings.NewReader("PWNED")))
	require.False(t, wd.Exists(escape))

	_, err := wd.Open(escape)
	require.Error(t, err)

	// Nothing may have been written outside the working folder
	_, err = os.Stat(filepath.Join(base, "escaped.txt"))
	require.True(t, os.IsNotExist(err), "must not write outside the working directory")
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

// TestWorkingDirectory_Remove confirms that Remove deletes the file from disk.
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

// TestWorkingDirectory_CloseRemovesFiles verifies that Close actually deletes the
// working files. The cache notifies its deletion listener asynchronously, so a
// Clear that only queues those notifications would be undone by the Close that
// immediately follows it, stranding every file on disk.
func TestWorkingDirectory_CloseRemovesFiles(t *testing.T) {

	dir := t.TempDir()
	wd := NewWorkingDirectory(dir, time.Minute, 10)

	require.NoError(t, wd.Write("a.txt", strings.NewReader("AAA")))
	require.NoError(t, wd.Write("b.txt", strings.NewReader("BBB")))

	wd.Close()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "Close must leave no files behind")
}

// TestWorkingDirectory_RemoveIsSynchronous verifies that the file is gone from disk
// as soon as Remove returns, rather than whenever the cache drains its write buffer.
func TestWorkingDirectory_RemoveIsSynchronous(t *testing.T) {

	wd := NewWorkingDirectory(t.TempDir(), time.Minute, 10)
	t.Cleanup(wd.Close)

	require.NoError(t, wd.Write("a.txt", strings.NewReader("AAA")))
	require.True(t, wd.Exists("a.txt"))

	wd.Remove("a.txt")
	require.False(t, wd.Exists("a.txt"))
}

// TestIsWorkingFileFor verifies that a working file is matched to the original it
// was generated from, and never to a different original that merely shares a prefix.
func TestIsWorkingFileFor(t *testing.T) {

	require.True(t, isWorkingFileFor("abc", "abc"))
	require.True(t, isWorkingFileFor("abc.webp", "abc"))
	require.True(t, isWorkingFileFor("abc_w300_h300.webp", "abc"))
	require.True(t, isWorkingFileFor("file.txt.txt", "file.txt"))

	require.False(t, isWorkingFileFor("abcdef.webp", "abc"))
	require.False(t, isWorkingFileFor("abcdef_w300.webp", "abc"))
	require.False(t, isWorkingFileFor("xyz.webp", "abc"))
	require.False(t, isWorkingFileFor("file.txt2.txt", "file.txt"))
}
