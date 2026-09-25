package mediaserver

import (
	"context"
	"strings"
	"time"
)

// defaultTimeout bounds an FFmpeg operation when the caller's context does not
// already carry a deadline.
const defaultTimeout = 5 * time.Minute

// Option configures a MediaServer when it is created with New.
type Option func(*options)

// options is the resolved configuration for a MediaServer.
type options struct {
	timeout         time.Duration // Bounds an FFmpeg operation when the caller sets no deadline
	allowedHosts    []string      // Hosts that cover images may come from (empty allows any public host)
	allowPrivateIPs bool          // If TRUE, then cover images may come from non-public IP addresses
}

// WithAllowedHosts restricts remote cover-image fetches to the named hosts, on top of the
// private-address block. When no hosts are supplied, any public host is allowed.
func WithAllowedHosts(hosts ...string) Option {
	return func(o *options) {
		for _, host := range hosts {
			o.allowedHosts = append(o.allowedHosts, strings.ToLower(host))
		}
	}
}

// WithAllowPrivateIPs controls whether remote cover-image fetches may connect to non-public
// IP addresses (loopback, private, link-local). The default is FALSE, which blocks them.
func WithAllowPrivateIPs(allow bool) Option {
	return func(o *options) {
		o.allowPrivateIPs = allow
	}
}

// newOptions returns the default options with any overrides applied.
func newOptions(opts ...Option) options {

	result := options{
		timeout: defaultTimeout,
	}

	for _, opt := range opts {
		opt(&result)
	}

	return result
}

// withTimeout derives an execution context from ctx, applying the configured timeout when ctx
// has no deadline of its own. The caller must always call the returned cancel.
func (o options) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {

	// A caller-supplied deadline is honored as-is
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, o.timeout)
}
