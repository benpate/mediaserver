package mediaserver

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benpate/derp"
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

// TestServe_ClosesProcessedFile confirms that every file Serve opens in the processed cache is
// closed again, so a remote (S3) cache does not leak a handle on every cache miss.
func TestServe_ClosesProcessedFile(t *testing.T) {

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "document", []byte("the original document contents"), 0777))

	processed := &countingFs{Fs: afero.NewMemMapFs()}
	working := NewWorkingDirectory(t.TempDir(), time.Minute, 100)
	t.Cleanup(working.Close)

	ms := New(originals, processed, working)
	filespec := FileSpec{Filename: "document", OriginalExtension: ".txt", Extension: ".txt"}

	require.NoError(t, ms.Serve(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/document", nil), filespec))
	require.Equal(t, int64(2), processed.opens.Load(), "Serve creates the cache file and then re-opens it")
	require.Equal(t, processed.opens.Load(), processed.closes.Load())
}

// TestEnsureProcessedFileExists_CloseFailure confirms that a cache file whose Close fails is
// reported as an error and removed, since a remote cache stores the upload at Close.
func TestEnsureProcessedFileExists_CloseFailure(t *testing.T) {

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "document", []byte("the original document contents"), 0777))

	processed := &countingFs{Fs: afero.NewMemMapFs(), failClose: true}
	ms := newTestServer(t, originals)
	ms.processed = processed

	filespec := FileSpec{Filename: "document", OriginalExtension: ".txt", Extension: ".txt"}
	require.Error(t, ms.ensureProcessedFileExists(t.Context(), filespec))

	exists, err := afero.Exists(processed, filespec.ProcessedPath())
	require.NoError(t, err)
	require.False(t, exists, "a cache file that failed to close must not be served later")
}

// countingFs wraps an afero.Fs to count the files it opens and closes, and can make every Close fail.
type countingFs struct {
	afero.Fs
	opens     atomic.Int64 // files returned by Open or Create
	closes    atomic.Int64 // calls to Close on those files
	failClose bool         // if TRUE, then every Close returns an error
}

// Open opens a file in the wrapped filesystem and counts it.
func (fs *countingFs) Open(name string) (afero.File, error) {
	return fs.track(fs.Fs.Open(name))
}

// Create creates a file in the wrapped filesystem and counts it.
func (fs *countingFs) Create(name string) (afero.File, error) {
	return fs.track(fs.Fs.Create(name))
}

// track counts a successfully opened file and wraps it so its Close is counted too.
func (fs *countingFs) track(file afero.File, err error) (afero.File, error) {

	if err != nil {
		return nil, err
	}

	fs.opens.Add(1)
	return countingFile{File: file, fs: fs}, nil
}

// countingFile is a file opened through a countingFs.
type countingFile struct {
	afero.File
	fs *countingFs // the filesystem that counts this file's Close
}

// Close closes the wrapped file, counts the call, and fails when the filesystem says to.
func (file countingFile) Close() error {

	file.fs.closes.Add(1)

	if err := file.File.Close(); err != nil {
		return err
	}

	if file.fs.failClose {
		return derp.Internal("mediaserver.countingFile.Close", "Simulated failure while closing file")
	}

	// Closed for business
	return nil
}
