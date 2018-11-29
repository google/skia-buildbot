package scheduling

import (
	"context"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
)

// Trigger all jobs with the given trigger name.
func triggerPeriodicJobsWithName(ctx context.Context, s *TaskScheduler, trigger string) error {
	// Obtain the TasksCfg at tip of master in each repo.
	cfgs := make(map[types.RepoState]*specs.TasksCfg, len(s.repos))
	for url, repo := range s.repos {
		head := repo.Get("master")
		rs := types.RepoState{
			Repo:     url,
			Revision: head.Hash,
		}
		cfg, err := s.taskCfgCache.ReadTasksCfg(ctx, rs)
		if err != nil {
			return err
		}
		cfgs[rs] = cfg
	}
	// Trigger the periodic tasks.
	sklog.Infof("Triggering %s tasks", trigger)
	jobs := []*types.Job{}
	for rs, cfg := range cfgs {
		for name, spec := range cfg.Jobs {
			if spec.Trigger == trigger {
				j, err := s.taskCfgCache.MakeJob(ctx, rs, name)
				if err != nil {
					return err
				}
				jobs = append(jobs, j)
			}
		}
	}
	return s.db.PutJobs(jobs)
}

// Register the nightly and weekly jobs to run.
func (s *TaskScheduler) registerPeriodicTriggers() {
	s.periodicTriggers.Register("nightly", func(ctx context.Context) error {
		return triggerPeriodicJobsWithName(ctx, s, "nightly")
	})
	s.periodicTriggers.Register("weekly", func(ctx context.Context) error {
		return triggerPeriodicJobsWithName(ctx, s, "weekly")
	})
}

// triggerPeriodicJobs triggers jobs at HEAD of the master branch in each repo
// for any files present in the trigger dir.
func (s *TaskScheduler) triggerPeriodicJobs(ctx context.Context) error {
	return s.periodicTriggers.RunPeriodicTriggers(ctx)
}
