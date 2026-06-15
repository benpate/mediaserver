package mediaserver

import (
	"context"
	"time"
)

// defaultTimeout bounds an FFmpeg operation when the caller's context does not
// already carry a deadline.
const defaultTimeout = 5 * time.Minute

// Option configures optional behavior for MediaServer operations that run FFmpeg.
type Option func(*options)

type options struct {
	timeout time.Duration
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
