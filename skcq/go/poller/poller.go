package poller

// Initializes and polls the various issue frameworks.

import (
	"context"
	"time"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/sklog"
)

// Start polls the different issue frameworks and populates DB and an in-memory object with that data.
// It hardcodes information about Skia's various clients. It may be possible to extract some/all of these into
// flags or YAML config files in the future.
func Start(ctx context.Context, pollInterval time.Duration, cr codereview_framework.Framework) error {

	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		if !*baseapp.Local {
			// Ignore the passed-in context; this allows us to continue running even if the
			// context is canceled due to transient errors.
			ctx = context.Background()
		}

		// Create a runID timestamp to associate all found issues with this poll iteration.
		runId := p.dbClient.GenerateRunId(time.Now())

		// Search all bug frameworks.
		for _, b := range bugFrameworks {
			if err := b.SearchClientAndPersist(ctx, p.dbClient, runId); err != nil {
				sklog.Errorf("Error when searching and saving issues: %s", err)
				return
			}
		}

		// We are done with this iteration. Add the runId timestamp to the DB.
		if err := p.dbClient.StoreRunId(context.Background(), runId); err != nil {
			sklog.Errorf("Could not store runId in DB: %s", err)
			return
		}

		p.openIssues.PrettyPrintOpenIssues()
	}, nil)

	return nil
}