package mediaserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// newNestedServer returns a MediaServer whose "processed" cache is an OS folder behind TWO
// BasePathFs layers, as Emissary builds it: one for the configured location, one per hostname.
// In that layering a file's Name() does not round-trip back through the outer BasePathFs, which
// the in-memory filesystems used elsewhere in these tests cannot show.
func newNestedServer(t *testing.T, original afero.Fs) (MediaServer, afero.Fs) {

	t.Helper()

	location := afero.NewBasePathFs(afero.NewOsFs(), t.TempDir())
	require.NoError(t, location.MkdirAll("localhost", 0777))
	processed := afero.NewBasePathFs(location, "localhost")

	working := NewWorkingDirectory(t.TempDir(), time.Minute, 100)
	t.Cleanup(working.Close)

	return New(original, processed, working), processed
}

// TestServe_FailureLeavesNoCacheFile confirms that a failed request leaves nothing in the processed
// cache, so the next request processes the file again.  A request that arrives before the original
// is stored must not poison the cache for every request after it.
func TestServe_FailureLeavesNoCacheFile(t *testing.T) {

	originals := afero.NewMemMapFs()
	ms, processed := newNestedServer(t, originals)
	filespec := FileSpec{Filename: "document", OriginalExtension: ".txt", Extension: ".txt", Cache: true}

	// The original is not stored yet, so this request fails
	early := httptest.NewRecorder()
	require.Error(t, ms.Serve(early, httptest.NewRequest(http.MethodGet, "/document", nil), filespec))

	exists, err := afero.Exists(processed, filespec.ProcessedPath())
	require.NoError(t, err)
	require.False(t, exists, "a failed request must remove the cache file it created")

	// Once the original arrives, the next request serves it
	content := []byte("the original document contents")
	require.NoError(t, afero.WriteFile(originals, "document", content, 0777))

	later := httptest.NewRecorder()
	require.NoError(t, ms.Serve(later, httptest.NewRequest(http.MethodGet, "/document", nil), filespec))
	require.Equal(t, content, later.Body.Bytes())
}

// TestServe_EmptyCacheFileIsReprocessed confirms that a zero-length file in the processed cache is
// treated as missing.  Earlier versions left one behind after every failed request, and serving it
// would return an empty body forever.
func TestServe_EmptyCacheFileIsReprocessed(t *testing.T) {

	originals := afero.NewMemMapFs()
	content := []byte("the original document contents")
	require.NoError(t, afero.WriteFile(originals, "document", content, 0777))

	ms, processed := newNestedServer(t, originals)
	filespec := FileSpec{Filename: "document", OriginalExtension: ".txt", Extension: ".txt", Cache: true}

	// A cache file left empty by an earlier failure
	require.NoError(t, processed.MkdirAll(filespec.ProcessedDir(), 0777))
	require.NoError(t, afero.WriteFile(processed, filespec.ProcessedPath(), []byte{}, 0777))

	recorder := httptest.NewRecorder()
	require.NoError(t, ms.Serve(recorder, httptest.NewRequest(http.MethodGet, "/document", nil), filespec))
	require.Equal(t, content, recorder.Body.Bytes())
}
