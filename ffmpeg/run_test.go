package ffmpeg

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// requireFFmpeg skips the calling test unless ffmpeg is both installed AND able
// to execute (it may be on the PATH yet fail to run, e.g. missing libraries).
func requireFFmpeg(t *testing.T) {
	t.Helper()

	if !IsInstalled {
		t.Skip("ffmpeg is not installed; skipping ffmpeg-dependent test")
	}

	if err := exec.Command("ffmpeg", "-version").Run(); err != nil {
		t.Skipf("ffmpeg is present but cannot run (%v); skipping", err)
	}
}

func TestRun_Success(t *testing.T) {
	requireFFmpeg(t)

	// "ffmpeg -version" exits zero, so Run succeeds.
	require.NoError(t, Run(context.Background(), "-version"))
}

func TestRun_Error(t *testing.T) {
	requireFFmpeg(t)

	// An unknown flag makes ffmpeg exit non-zero, so Run returns an error.
	require.Error(t, Run(context.Background(), "-definitely-not-a-real-flag"))
}

func TestRun_ContextCancelled(t *testing.T) {
	requireFFmpeg(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before running

	require.Error(t, Run(ctx, "-version"))
}
