//go:build localonly

package mediaserver

import (
	"os"
	"testing"
)

// TestTempDir logs the OS temp directory, and runs only with the localonly build tag.
func TestTempDir(t *testing.T) {
	t.Log(os.TempDir())
}
