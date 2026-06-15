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

// init registers the audio/video extensions the tests rely on, so that
// mime.TypeByExtension resolves them deterministically regardless of the host
// operating system's mime database.
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

// requireWorkingFFmpeg skips the calling test unless ffmpeg is both installed
// AND able to execute. Checking IsInstalled alone is not enough: ffmpeg may be
// on the PATH yet fail to run (e.g. missing shared libraries), and CI machines
// frequently have no ffmpeg at all.
func requireWorkingFFmpeg(t *testing.T) {
	t.Helper()

	if !ffmpeg.IsInstalled {
		t.Skip("ffmpeg is not installed; skipping ffmpeg-dependent test")
	}

	if err := exec.Command("ffmpeg", "-version").Run(); err != nil {
		t.Skipf("ffmpeg is present but cannot run (%v); skipping ffmpeg-dependent test", err)
	}
}

// newTestServer returns a MediaServer backed by in-memory "original" and
// "processed" filesystems and a working directory in a temporary folder. The
// working directory is closed automatically when the test finishes.
func newTestServer(t *testing.T, original afero.Fs) MediaServer {
	t.Helper()

	if original == nil {
		original = afero.NewMemMapFs()
	}

	processed := afero.NewMemMapFs()
	working := NewWorkingDirectory(t.TempDir(), time.Minute, 100)
	t.Cleanup(working.Close)

	return New(original, processed, &working)
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
