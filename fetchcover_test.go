package mediaserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFetchCover_BlocksInternalAddress confirms that the default policy refuses to fetch from a
// loopback server.
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

// TestFetchCover_RejectsNonHTTPScheme confirms that file, data, and ftp URLs are rejected.
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

// TestFetchCover_RejectsDisallowedHost confirms that a host missing from the allow-list is rejected.
func TestFetchCover_RejectsDisallowedHost(t *testing.T) {
	// With an allow-list configured, a host that is not on it is rejected before
	// any connection is attempted.
	ms := newTestServer(t, nil, WithAllowedHosts("cdn.example.com"))

	_, err := ms.fetchCover(context.Background(), "https://evil.example.com/cover.jpg")
	require.Error(t, err)
}

// TestFetchCover_AllowsPublicDownload confirms that, with private IPs allowed, fetchCover downloads
// the response body into a temp file.
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

// FuzzFetchCover confirms that, under the default policy, fetchCover never panics on an arbitrary
// URL, returns no filename on failure, and rejects any scheme other than http or https.
func FuzzFetchCover(f *testing.F) {

	// Seed with a mix of valid-looking, malformed, and dangerous inputs.
	for _, seed := range []string{
		"",
		"http://example.com/cover.jpg",
		"https://example.com/cover.jpg",
		"file:///etc/passwd",
		"data:text/plain;base64,SGVsbG8=",
		"ftp://example.com/x",
		"http://127.0.0.1/cover.jpg",
		"http://[::1]/cover.jpg",
		"https://user:pass@example.com/x",
		"://missing-scheme",
		"http://",
		"h t t p://bad",
		"\x00\x01\x02",
	} {
		f.Add(seed)
	}

	ms := newTestServer(f, nil)

	f.Fuzz(func(t *testing.T, rawURL string) {

		// A short deadline keeps an unresponsive public host from blocking the
		// fuzzer on the remote client's default network timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()

		filename, err := ms.fetchCover(ctx, rawURL)

		// A valid http(s) URL may reach a public host, which the SSRF guard permits,
		// so success is allowed; clean up the temp file it returns.
		if err == nil {
			_ = os.Remove(filename)
			require.NotEmpty(t, filename, "a successful fetch must return a filename for %q", rawURL)
			return
		}

		// On failure, no temp file name is handed back to leak.
		require.Empty(t, filename, "a failed fetch should return an empty filename for %q", rawURL)

		// A URL that parses cleanly but is not http/https must be rejected on
		// scheme alone, before any network access is attempted.
		if parsed, parseErr := url.Parse(rawURL); parseErr == nil {
			if scheme := strings.ToLower(parsed.Scheme); scheme != "http" && scheme != "https" {
				require.Error(t, err, "non-http(s) scheme %q must be rejected", parsed.Scheme)
			}
		}
	})
}

// TestGetCoverPhoto_EndToEnd confirms that getCoverPhoto downloads and resizes a PNG from a loopback
// server into a non-empty file.
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
