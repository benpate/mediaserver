//go:build localonly

package mediaserver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCoverPhoto(t *testing.T) {

	filename, err := getCoverPhoto("http://localhost/6692c69bfe80a9aacf125b0d/attachments/6723b7b74aa88ca07dc8614e")

	require.Nil(t, err)
	t.Log(filename)
}
