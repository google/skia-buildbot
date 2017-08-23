package scheduling

import (
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

// Trigger all jobs with the given trigger name.
func triggerPeriodicJobsWithName(s *TaskScheduler, trigger string) error {
	// Obtain the TasksCfg at tip of master in each repo.
	cfgs := make(map[db.RepoState]*specs.TasksCfg, len(s.repos))
	for url, repo := range s.repos {
		head := repo.Get("master")
		rs := db.RepoState{
			Repo:     url,
			Revision: head.Hash,
		}
		cfg, err := s.taskCfgCache.ReadTasksCfg(rs)
		if err != nil {
			return err
		}
		cfgs[rs] = cfg
	}
	// Trigger the periodic tasks.
	sklog.Infof("Triggering %s tasks", trigger)
	jobs := []*db.Job{}
	for rs, cfg := range cfgs {
		for name, spec := range cfg.Jobs {
			if spec.Trigger == trigger {
				j, err := s.taskCfgCache.MakeJob(rs, name)
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
	s.periodicTriggers.Register("nightly", func() error {
		return triggerPeriodicJobsWithName(s, "nightly")
	})
	s.periodicTriggers.Register("weekly", func() error {
		return triggerPeriodicJobsWithName(s, "weekly")
	})
}

// triggerPeriodicJobs triggers jobs at HEAD of the master branch in each repo
// for any files present in the trigger dir.
func (s *TaskScheduler) triggerPeriodicJobs() error {
	return s.periodicTriggers.RunPeriodicTriggers()
}
