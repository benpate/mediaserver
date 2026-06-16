//go:build localonly

package mediaserver

import (
	"os"
	"testing"
)

func TestTempDir(t *testing.T) {
	t.Log(os.TempDir())
}
