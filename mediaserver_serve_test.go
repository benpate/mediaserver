package mediaserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestServeOriginal(t *testing.T) {
	originals := afero.NewMemMapFs()
	content := []byte("raw original bytes")
	require.NoError(t, afero.WriteFile(originals, "raw.bin", content, 0777))

	ms := newTestServer(t, originals)
	recorder := httptest.NewRecorder()

	err := ms.ServeOriginal(recorder, httptest.NewRequest(http.MethodGet, "/raw.bin", nil), "raw.bin")
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/octet-stream", recorder.Header().Get("Content-Type"))
	require.Equal(t, content, recorder.Body.Bytes())
}

func TestServeOriginal_Missing(t *testing.T) {
	ms := newTestServer(t, nil)
	recorder := httptest.NewRecorder()

	err := ms.ServeOriginal(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil), "missing")
	require.Error(t, err)
}

// TestServe_CopyThrough exercises the full Serve pipeline (working file -> cache
// -> Process) end-to-end. It uses a non-media file so the whole flow runs
// without ffmpeg.
func TestServe_CopyThrough(t *testing.T) {
	originals := afero.NewMemMapFs()
	content := []byte("the original document contents")
	require.NoError(t, afero.WriteFile(originals, "document", content, 0777))

	ms := newTestServer(t, originals)
	filespec := FileSpec{Filename: "document", OriginalExtension: ".txt", Extension: ".txt", Cache: true}

	recorder := httptest.NewRecorder()
	err := ms.Serve(recorder, httptest.NewRequest(http.MethodGet, "/document", nil), filespec)
	require.NoError(t, err)

	require.Equal(t, content, recorder.Body.Bytes())
	require.Equal(t, "IMMUTABLE", recorder.Header().Get("ETag"))
	require.Equal(t, "public, max-age=86400, immutable", recorder.Header().Get("Cache-Control"))
}

// TestServe_CopyThrough_UsesCache confirms that a second Serve call reuses the
// already-processed working file (the original can even be removed in between).
func TestServe_CopyThrough_UsesCache(t *testing.T) {
	originals := afero.NewMemMapFs()
	content := []byte("cached document contents")
	require.NoError(t, afero.WriteFile(originals, "document", content, 0777))

	ms := newTestServer(t, originals)
	filespec := FileSpec{Filename: "document", OriginalExtension: ".txt", Extension: ".txt", Cache: true}

	// First call processes and caches the file.
	first := httptest.NewRecorder()
	require.NoError(t, ms.Serve(first, httptest.NewRequest(http.MethodGet, "/document", nil), filespec))
	require.Equal(t, content, first.Body.Bytes())

	// Second call should serve from the existing working file.
	second := httptest.NewRecorder()
	require.NoError(t, ms.Serve(second, httptest.NewRequest(http.MethodGet, "/document", nil), filespec))
	require.Equal(t, content, second.Body.Bytes())
}

func TestServe_ProcessingError(t *testing.T) {
	// A media file with ffmpeg unavailable fails during the processing step,
	// and the error propagates out through Serve.
	original := ffmpegInstalled
	ffmpegInstalled = func() bool { return false }
	t.Cleanup(func() { ffmpegInstalled = original })

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "photo.jpg", []byte("not really a jpg"), 0777))

	ms := newTestServer(t, originals)
	filespec := FileSpec{Filename: "photo.jpg", OriginalExtension: ".jpg", Extension: ".jpg", Cache: true}

	recorder := httptest.NewRecorder()
	require.Error(t, ms.Serve(recorder, httptest.NewRequest(http.MethodGet, "/photo.jpg", nil), filespec))
}

func TestServe_Image(t *testing.T) {
	requireWorkingFFmpeg(t)

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "photo.png", makePNG(t, 64, 48), 0777))

	ms := newTestServer(t, originals)
	filespec := FileSpec{Filename: "photo.png", OriginalExtension: ".png", Extension: ".png", Width: 32, Height: 32, Cache: true}

	recorder := httptest.NewRecorder()
	err := ms.Serve(recorder, httptest.NewRequest(http.MethodGet, "/photo.png", nil), filespec)
	require.NoError(t, err)
	require.NotEmpty(t, recorder.Body.Bytes())
}
