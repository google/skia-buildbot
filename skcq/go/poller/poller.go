package poller

// Initializes and polls the various issue frameworks.

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
)

const (
	LivenessMetric = "skcq_be"
)

// Start polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func Start(ctx context.Context, pollInterval time.Duration, cr codereview.CodeReview) error {

	liveness := metrics2.NewLiveness(LivenessMetric)
	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		if !*baseapp.Local {
			// Ignore the passed-in context; this allows us to continue running even if the
			// context is canceled due to transient errors.
			ctx = context.Background()
		}

		fmt.Println("POLLING!")
		_, err := cr.Search(ctx)
		if err != nil {
			sklog.Errorf("Error when searching for issues: %s", err)
		} else {
			// This should only be done if there are no errors.
			liveness.Reset()
		}
		fmt.Println("DONE")

	}, nil)

	return nil
}
