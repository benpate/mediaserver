package mediaserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestDelete(t *testing.T) {
	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "file.txt", []byte("data"), 0777))
	ms := newTestServer(t, originals)

	require.NoError(t, ms.Delete("file.txt"))

	exists, err := afero.Exists(originals, "file.txt")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestDelete_OriginalError(t *testing.T) {
	// Removing from a read-only filesystem fails.
	readOnly := afero.NewReadOnlyFs(afero.NewMemMapFs())
	ms := newTestServer(t, readOnly)

	require.Error(t, ms.Delete("missing.txt"))
}

func TestDelete_ProcessedError(t *testing.T) {
	// The original is removable, but the processed filesystem is read-only, so
	// removing the cached files fails.
	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "file.txt", []byte("data"), 0777))

	working := NewWorkingDirectory(t.TempDir(), time.Minute, 100)
	t.Cleanup(working.Close)
	ms := New(originals, afero.NewReadOnlyFs(afero.NewMemMapFs()), working)

	require.Error(t, ms.Delete("file.txt"))
}

// TestDelete_PurgesWorkingDirectory verifies that Delete removes the local working
// copies as well as the original and the cache. Serve consults the working
// directory first, so a leftover copy would keep serving a deleted file.
func TestDelete_PurgesWorkingDirectory(t *testing.T) {

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "file.txt", []byte("secret"), 0777))
	ms := newTestServer(t, originals)

	filespec := NewFileSpec()
	filespec.Filename = "file.txt"
	filespec.OriginalExtension = ".txt"
	filespec.Extension = ".txt"

	// The first request populates the processed cache and the working directory
	first := httptest.NewRecorder()
	require.NoError(t, ms.Serve(first, httptest.NewRequest(http.MethodGet, "/file.txt", nil), filespec))
	require.Equal(t, "secret", first.Body.String())
	require.True(t, ms.working.Exists(filespec.WorkingFilename()))

	// Delete the file
	require.NoError(t, ms.Delete("file.txt"))
	require.False(t, ms.working.Exists(filespec.WorkingFilename()), "working copy must not survive Delete")

	// A second request must now fail, rather than serving the deleted content
	second := httptest.NewRecorder()
	err := ms.Serve(second, httptest.NewRequest(http.MethodGet, "/file.txt", nil), filespec)
	require.Error(t, err)
	require.NotContains(t, second.Body.String(), "secret")
}

// TestDelete_LeavesOtherWorkingFiles verifies that purging one original does not
// take out a different original whose name merely starts the same way.
func TestDelete_LeavesOtherWorkingFiles(t *testing.T) {

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "abc", []byte("one"), 0777))
	require.NoError(t, afero.WriteFile(originals, "abcdef", []byte("two"), 0777))
	ms := newTestServer(t, originals)

	serve := func(name string) FileSpec {
		filespec := NewFileSpec()
		filespec.Filename = name
		filespec.OriginalExtension = ".txt"
		filespec.Extension = ".txt"
		require.NoError(t, ms.Serve(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/"+name, nil), filespec))
		return filespec
	}

	shortSpec := serve("abc")
	longSpec := serve("abcdef")

	require.NoError(t, ms.Delete("abc"))

	require.False(t, ms.working.Exists(shortSpec.WorkingFilename()))
	require.True(t, ms.working.Exists(longSpec.WorkingFilename()), "unrelated original must survive")
}
