package mediaserver

import (
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestPut(t *testing.T) {
	originals := afero.NewMemMapFs()
	ms := newTestServer(t, originals)

	require.NoError(t, ms.Put("upload.txt", strings.NewReader("file data")))

	contents, err := afero.ReadFile(originals, "upload.txt")
	require.NoError(t, err)
	require.Equal(t, "file data", string(contents))
}

func TestPut_CreateError(t *testing.T) {
	// A read-only filesystem cannot create the destination file.
	readOnly := afero.NewReadOnlyFs(afero.NewMemMapFs())
	ms := newTestServer(t, readOnly)

	require.Error(t, ms.Put("upload.txt", strings.NewReader("file data")))
}

func TestPut_CopyError(t *testing.T) {
	// The destination is created, but copying from a failing reader errors out.
	ms := newTestServer(t, afero.NewMemMapFs())
	require.Error(t, ms.Put("upload.txt", errorReader{}))
}
