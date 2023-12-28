// Package maintenance runs long running processes for a single perf instance.
package maintenance

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/builders"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracing"
)

const (
	// How often to update the git repo from origin.
	gitRepoUpdatePeriod = time.Minute
)

// Start all the long running processes. This function does not return if all
// the processes started correctly.
func Start(ctx context.Context, flags config.MaintenanceFlags, instanceConfig *config.InstanceConfig) error {
	if err := tracing.Init(flags.Local, instanceConfig); err != nil {
		return skerr.Wrapf(err, "Start tracing.")
	}

	// New perfgit.Git.
	g, err := builders.NewPerfGitFromConfig(ctx, flags.Local, instanceConfig)
	if err != nil {
		return skerr.Wrapf(err, "Build perfGit instance.")
	}
	// Start a background process that periodically adds new commits to the
	// database.
	g.StartBackgroundPolling(ctx, gitRepoUpdatePeriod)

	select {}
}
