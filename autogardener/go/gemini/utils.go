package gemini

import (
	"time"

	"github.com/cenkalti/backoff"
	"go.skia.org/infra/go/sklog"
)

func doBackoff(opName string, fn func() error) error {
	// These are default values at the time of writing, but we lay them out
	// explicitly for clarity.
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 500 * time.Millisecond
	b.RandomizationFactor = 0.5
	b.Multiplier = 1.5
	b.MaxInterval = 60 * time.Second
	b.MaxElapsedTime = 15 * time.Minute
	return backoff.RetryNotify(fn, b, func(err error, d time.Duration) {
		sklog.Warningf("%s failed; retrying in %s: %s", opName, d, err)
	})
}
