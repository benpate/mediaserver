package mediaserver

import (
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestDelete(t *testing.T) {
	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "file.txt", []byte("data"), 0777))
	ms := newTestServer(t, originals)

	require.NoError(t, ms.Delete("file.txt"))

	exists, err := afero.Exists(originals, "file.txt")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestDelete_OriginalError(t *testing.T) {
	// Removing from a read-only filesystem fails.
	readOnly := afero.NewReadOnlyFs(afero.NewMemMapFs())
	ms := newTestServer(t, readOnly)

	require.Error(t, ms.Delete("missing.txt"))
}

func TestDelete_ProcessedError(t *testing.T) {
	// The original is removable, but the processed filesystem is read-only, so
	// removing the cached files fails.
	originals := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(originals, "file.txt", []byte("data"), 0777))

	working := NewWorkingDirectory(t.TempDir(), time.Minute, 100)
	t.Cleanup(working.Close)
	ms := New(originals, afero.NewReadOnlyFs(afero.NewMemMapFs()), working)

	require.Error(t, ms.Delete("file.txt"))
}
