package mediaserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestProcess_NonMediaCopiesThrough(t *testing.T) {
	// A non-media file is copied verbatim, with no call to ffmpeg.
	originals := afero.NewMemMapFs()
	content := []byte("this is a plain text document")
	require.NoError(t, afero.WriteFile(originals, "doc.txt", content, 0777))

	ms := newTestServer(t, originals)
	filespec := FileSpec{Filename: "doc.txt", OriginalExtension: ".txt", Extension: ".txt"}

	var output bytes.Buffer
	require.NoError(t, ms.Process(context.Background(), filespec, &output))
	require.Equal(t, content, output.Bytes())
}

func TestProcess_MissingOriginal(t *testing.T) {
	ms := newTestServer(t, nil)
	filespec := FileSpec{Filename: "missing.txt", OriginalExtension: ".txt", Extension: ".txt"}

	var output bytes.Buffer
	require.Error(t, ms.Process(context.Background(), filespec, &output))
}

func TestProcess_MediaWithoutFFmpeg(t *testing.T) {
	// A media file cannot be processed when ffmpeg is not installed.
	original := ffmpegInstalled
	ffmpegInstalled = func() bool { return false }
	t.Cleanup(func() { ffmpegInstalled = original })

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "photo.jpg", []byte("not really a jpg"), 0777))

	ms := newTestServer(t, originals)
	filespec := FileSpec{Filename: "photo.jpg", OriginalExtension: ".jpg", Extension: ".jpg"}

	var output bytes.Buffer
	require.Error(t, ms.Process(context.Background(), filespec, &output))
}

func TestProcess_ContextCancelled(t *testing.T) {
	// A cancelled context passed to Process aborts the FFmpeg run.
	requireWorkingFFmpeg(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "photo.png", makePNG(t, 64, 48), 0777))

	ms := newTestServer(t, originals)
	filespec := FileSpec{Filename: "photo.png", OriginalExtension: ".png", Extension: ".png", Width: 32, Height: 32}

	var output bytes.Buffer
	require.Error(t, ms.Process(ctx, filespec, &output))
}

func TestProcess_Image(t *testing.T) {
	requireWorkingFFmpeg(t)

	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "photo.png", makePNG(t, 64, 48), 0777))

	ms := newTestServer(t, originals)
	filespec := FileSpec{Filename: "photo.png", OriginalExtension: ".png", Extension: ".png", Width: 32, Height: 32}

	var output bytes.Buffer
	require.NoError(t, ms.Process(context.Background(), filespec, &output))
	require.NotEmpty(t, output.Bytes())
}

func TestProcessArguments(t *testing.T) {

	ctx := context.Background()

	t.Run("no metadata", func(t *testing.T) {
		ms := newTestServer(t, nil)
		args, cleanup := ms.processArguments(ctx, FileSpec{Extension: ".jpg"}, "in.jpg", "out.jpg")
		t.Cleanup(cleanup)

		require.Equal(t, []string{"-y", "-i", "in.jpg", "-c:v", "mjpeg", "out.jpg"}, args)
	})

	t.Run("non-cover metadata uses key=value with no quotes", func(t *testing.T) {
		ms := newTestServer(t, nil)
		filespec := NewFileSpec()
		filespec.Extension = ".jpg"
		filespec.Metadata["artist"] = "Beatles"
		filespec.Metadata["comment"] = "line1\nline2"

		args, cleanup := ms.processArguments(ctx, filespec, "in.jpg", "out.jpg")
		t.Cleanup(cleanup)

		// Values are passed verbatim as key=value (NOT shell-quoted), and embedded
		// newlines are flattened so they cannot break ffmpeg's metadata parsing.
		require.Contains(t, args, "artist=Beatles")
		require.NotContains(t, args, `artist="Beatles"`)
		require.Contains(t, args, `comment=line1\nline2`)
	})

	t.Run("blocked cover URL is skipped, not fatal", func(t *testing.T) {
		// The cover is served on loopback; with the default (secure) policy the
		// SSRF guard blocks it, so getCoverPhoto fails and the cover is omitted.
		png := makePNG(t, 8, 8)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(png)
		}))
		t.Cleanup(server.Close)

		ms := newTestServer(t, nil)
		filespec := NewFileSpec()
		filespec.Extension = ".mp3"
		filespec.Metadata["cover"] = server.URL

		args, cleanup := ms.processArguments(ctx, filespec, "in.mp3", "out.mp3")
		t.Cleanup(cleanup)

		require.NotContains(t, args, "-map") // no cover art was added
	})

	t.Run("cover art adds a second input", func(t *testing.T) {
		requireWorkingFFmpeg(t)

		png := makePNG(t, 64, 64)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		}))
		t.Cleanup(server.Close)

		ms := newTestServer(t, nil, WithAllowPrivateIPs(true))
		filespec := NewFileSpec()
		filespec.Extension = ".mp3"
		filespec.Metadata["cover"] = server.URL

		args, cleanup := ms.processArguments(ctx, filespec, "in.mp3", "out.mp3")

		// Cover art is mapped in as a second input stream...
		require.Subset(t, args, []string{"-map", "0:a", "1:v", "copy"})

		// ...and cleanup removes the downloaded cover temp file.
		require.NotPanics(t, cleanup)
	})
}
