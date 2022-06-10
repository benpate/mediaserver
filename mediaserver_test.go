package mediaserver

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestMediaServer(t *testing.T) {

	mock_originals := afero.NewMemMapFs()
	mock_cache := afero.NewMemMapFs()

	m := New(mock_originals, mock_cache)

	require.NotNil(t, m)
}
