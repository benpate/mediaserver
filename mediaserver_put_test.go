package mediaserver

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// TestPut confirms that Put writes the reader's contents to the original filesystem.
func TestPut(t *testing.T) {
	originals := afero.NewMemMapFs()
	ms := newTestServer(t, originals)

	require.NoError(t, ms.Put("upload.txt", strings.NewReader("file data")))

	contents, err := afero.ReadFile(originals, "upload.txt")
	require.NoError(t, err)
	require.Equal(t, "file data", string(contents))
}

// TestPut_CreateError confirms that Put fails when the destination file cannot be created.
func TestPut_CreateError(t *testing.T) {
	// A read-only filesystem cannot create the destination file.
	readOnly := afero.NewReadOnlyFs(afero.NewMemMapFs())
	ms := newTestServer(t, readOnly)

	require.Error(t, ms.Put("upload.txt", strings.NewReader("file data")))
}

// TestPut_CopyError confirms that Put fails when its reader returns an error.
func TestPut_CopyError(t *testing.T) {
	// The destination is created, but copying from a failing reader errors out.
	ms := newTestServer(t, afero.NewMemMapFs())
	require.Error(t, ms.Put("upload.txt", errorReader{}))
}
