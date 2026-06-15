package mediaserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewOptions_Defaults(t *testing.T) {
	require.Equal(t, defaultTimeout, newOptions().timeout)
}

func TestNewOptions_AppliesOverrides(t *testing.T) {
	// The apply loop runs each Option against the defaults.
	applied := false
	newOptions(func(o *options) { applied = true })
	require.True(t, applied)
}

func TestOptions_WithTimeout_NoDeadline(t *testing.T) {
	// A context with no deadline gets the configured timeout as a safety net.
	ctx, cancel := options{timeout: time.Minute}.withTimeout(context.Background())
	t.Cleanup(cancel)

	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.WithinDuration(t, time.Now().Add(time.Minute), deadline, 5*time.Second)
}

func TestOptions_WithTimeout_HonorsCallerDeadline(t *testing.T) {
	// A caller-supplied deadline is honored as-is, not overridden by the timeout.
	callerDeadline := time.Now().Add(time.Hour)
	parent, cancelParent := context.WithDeadline(context.Background(), callerDeadline)
	t.Cleanup(cancelParent)

	ctx, cancel := options{timeout: time.Minute}.withTimeout(parent)
	t.Cleanup(cancel)

	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.Equal(t, callerDeadline, deadline)
}
