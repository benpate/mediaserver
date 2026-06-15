package mediaserver

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestMediaServer(t *testing.T) {

	mockOriginals := afero.NewMemMapFs()
	mockCache := afero.NewMemMapFs()
	mockWorking := NewWorkingDirectory(os.TempDir(), 1*time.Minute, 100)

	m := New(mockOriginals, mockCache, mockWorking)

	require.NotNil(t, m)
}
