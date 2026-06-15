package mediaserver

import (
	"bytes"
	"context"
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
