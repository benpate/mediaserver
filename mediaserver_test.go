package mediaserver

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestMediaServer(t *testing.T) {

	mock_originals := afero.NewMemMapFs()
	mock_cache := afero.NewMemMapFs()
	mock_working := NewWorkingDirectory(os.TempDir(), 1*time.Minute, 100)

	m := New(mock_originals, mock_cache, &mock_working)

	require.NotNil(t, m)
}
