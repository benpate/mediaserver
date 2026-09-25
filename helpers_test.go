package mediaserver

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime"
	"os/exec"
	"testing"
	"time"

	"github.com/benpate/mediaserver/ffmpeg"
	"github.com/spf13/afero"
)

// init registers the audio and video extensions the tests rely on, so mime.TypeByExtension
// resolves them the same way regardless of the host's mime database.
func init() {
	for ext, mimeType := range map[string]string{
		".mp3":  "audio/mpeg",
		".aac":  "audio/aac",
		".flac": "audio/flac",
		".m4a":  "audio/mp4",
		".ogg":  "audio/ogg",
		".opus": "audio/opus",
		".wav":  "audio/wav",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".mov":  "video/quicktime",
	} {
		_ = mime.AddExtensionType(ext, mimeType)
	}
}

// requireWorkingFFmpeg skips the calling test unless ffmpeg is both installed and able to run.
func requireWorkingFFmpeg(t *testing.T) {
	t.Helper()

	if !ffmpeg.IsInstalled() {
		t.Skip("ffmpeg is not installed; skipping ffmpeg-dependent test")
	}

	// ffmpeg may be on the PATH yet fail to run, e.g. with missing shared libraries
	if err := exec.Command("ffmpeg", "-version").Run(); err != nil {
		t.Skipf("ffmpeg is present but cannot run (%v); skipping ffmpeg-dependent test", err)
	}
}

// newTestServer returns a MediaServer over the given (or an in-memory) original filesystem, an
// in-memory processed cache, and a temporary working directory that closes when the test ends.
func newTestServer(t testing.TB, original afero.Fs, opts ...Option) MediaServer {
	t.Helper()

	if original == nil {
		original = afero.NewMemMapFs()
	}

	processed := afero.NewMemMapFs()
	working := NewWorkingDirectory(t.TempDir(), time.Minute, 100)
	t.Cleanup(working.Close)

	return New(original, processed, working, opts...)
}

// makePNG returns the bytes of a valid solid-color PNG of the given dimensions.
func makePNG(t *testing.T, width int, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 50, B: 50, A: 255})
		}
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("unable to encode test PNG: %v", err)
	}

	return buffer.Bytes()
}
