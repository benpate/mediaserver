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
	timeout         time.Duration
	allowedHosts    []string
	allowPrivateIPs bool
}

// WithAllowedHosts restricts remote cover-image fetches to the named hosts (in
// addition to the always-on block on internal/private addresses). When no hosts
// are supplied, any public host is allowed.
func WithAllowedHosts(hosts ...string) Option {
	return func(o *options) {
		for _, host := range hosts {
			o.allowedHosts = append(o.allowedHosts, strings.ToLower(host))
		}
	}
}

// WithAllowPrivateIPs controls whether remote cover-image fetches may connect to
// non-public IP addresses (loopback, private, link-local, etc.). The default is
// FALSE, so such addresses are blocked to guard against SSRF. Set it to TRUE only
// when intentionally fetching from an internal or localhost service.
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

// withTimeout derives an execution context from ctx. A caller-supplied deadline
// is honored as-is; otherwise the default timeout is applied as a safety net so
// a runaway FFmpeg process cannot hang forever. The returned cancel must always
// be called.
func (o options) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {

	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, o.timeout)
}
