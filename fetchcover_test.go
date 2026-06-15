package mediaserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchCover_BlocksInternalAddress(t *testing.T) {
	// An httptest server listens on loopback; with the default (secure) policy the
	// remote client must refuse to connect to it. This is the core SSRF
	// protection, exercised with no external network and no ffmpeg.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("should never be reached"))
	}))
	t.Cleanup(server.Close)

	ms := newTestServer(t, nil)

	_, err := ms.fetchCover(context.Background(), server.URL)
	require.Error(t, err)
}

func TestFetchCover_RejectsNonHTTPScheme(t *testing.T) {
	ms := newTestServer(t, nil)

	for _, rawURL := range []string{
		"file:///etc/passwd",
		"data:text/plain;base64,SGVsbG8=",
		"ftp://example.com/x",
	} {
		_, err := ms.fetchCover(context.Background(), rawURL)
		require.Error(t, err, "url=%s", rawURL)
	}
}

func TestFetchCover_RejectsDisallowedHost(t *testing.T) {
	// With an allow-list configured, a host that is not on it is rejected before
	// any connection is attempted.
	ms := newTestServer(t, nil, WithAllowedHosts("cdn.example.com"))

	_, err := ms.fetchCover(context.Background(), "https://evil.example.com/cover.jpg")
	require.Error(t, err)
}

func TestFetchCover_AllowsPublicDownload(t *testing.T) {
	// WithAllowPrivateIPs(true) permits loopback, so we can verify the happy path
	// (download into a size-limited temp file) against a local httptest server.
	body := []byte("pretend this is image bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	ms := newTestServer(t, nil, WithAllowPrivateIPs(true))

	filename, err := ms.fetchCover(context.Background(), server.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(filename) })

	contents, err := os.ReadFile(filename)
	require.NoError(t, err)
	require.Equal(t, body, contents)
}

func TestGetCoverPhoto_EndToEnd(t *testing.T) {
	// Serve a real PNG from a loopback server, allow loopback for this test, and
	// confirm getCoverPhoto downloads + resizes it into a non-empty file.
	requireWorkingFFmpeg(t)

	png := makePNG(t, 200, 200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	t.Cleanup(server.Close)

	ms := newTestServer(t, nil, WithAllowPrivateIPs(true))

	filename, err := ms.getCoverPhoto(context.Background(), server.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(filename) })

	info, err := os.Stat(filename)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}
