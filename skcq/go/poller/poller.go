package poller

// Initializes and polls the various issue frameworks.

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/skcq/go/codereview"
)

// Start polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func Start(ctx context.Context, pollInterval time.Duration, cr codereview.CodeReview) error {

	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		if !*baseapp.Local {
			// Ignore the passed-in context; this allows us to continue running even if the
			// context is canceled due to transient errors.
			ctx = context.Background()
		}

		fmt.Println("POLLING!")
		cr.Search()
		fmt.Println("DONE")
	}, nil)

	return nil
}
