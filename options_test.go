package mediaserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNewOptions_Defaults confirms that newOptions starts with the default timeout.
func TestNewOptions_Defaults(t *testing.T) {
	require.Equal(t, defaultTimeout, newOptions().timeout)
}

// TestWithAllowedHosts confirms that WithAllowedHosts records its hosts in lower case.
func TestWithAllowedHosts(t *testing.T) {
	// The option records the hosts (lower-cased) for the remote client to enforce.
	o := newOptions(WithAllowedHosts("CDN.Example.com", "images.example.NET"))
	require.Equal(t, []string{"cdn.example.com", "images.example.net"}, o.allowedHosts)
}

// TestWithAllowPrivateIPs confirms that private IPs are blocked by default and allowed by
// WithAllowPrivateIPs(true).
func TestWithAllowPrivateIPs(t *testing.T) {
	// The default is secure: private IPs are blocked.
	require.False(t, newOptions().allowPrivateIPs)

	// The option opts in to allowing them.
	require.True(t, newOptions(WithAllowPrivateIPs(true)).allowPrivateIPs)
}

// TestNewOptions_AppliesOverrides confirms that newOptions calls each Option it is given.
func TestNewOptions_AppliesOverrides(t *testing.T) {
	// The apply loop runs each Option against the defaults.
	applied := false
	newOptions(func(_ *options) { applied = true })
	require.True(t, applied)
}

// TestOptions_WithTimeout_NoDeadline confirms that withTimeout gives a context with no deadline the
// configured timeout.
func TestOptions_WithTimeout_NoDeadline(t *testing.T) {
	// A context with no deadline gets the configured timeout as a safety net.
	ctx, cancel := options{timeout: time.Minute}.withTimeout(context.Background())
	t.Cleanup(cancel)

	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	require.WithinDuration(t, time.Now().Add(time.Minute), deadline, 5*time.Second)
}

// TestOptions_WithTimeout_HonorsCallerDeadline confirms that withTimeout keeps a deadline the caller
// already set.
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
