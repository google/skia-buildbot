package scheduling

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	// TRIGGER_DIRNAME is the name of the directory containing files
	// indicating that periodic jobs should be triggered. These files are
	// created by the systemd task-scheduler-trigger-*.service.
	TRIGGER_DIRNAME = "periodic-job-trigger"
)

// findAndParseTriggerFiles returns the base filenames for each file in
// triggerDir.
func findAndParseTriggerFiles(triggerDir string) ([]string, error) {
	dir, err := os.Open(triggerDir)
	if err != nil {
		return nil, fmt.Errorf("Unable to read trigger directory %s: %s", triggerDir, err)
	}
	defer util.Close(dir)
	files, err := dir.Readdirnames(1)
	if err == io.EOF {
		return []string{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("Unable to list trigger directory %s: %s", triggerDir, err)
	}
	return files, nil
}

// deleteTriggerFile removes the given trigger file indicating that the backup
// succeeded.
func deleteTriggerFile(triggerDir, basename string) error {
	filename := path.Join(triggerDir, basename)
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("Unable to remove trigger file %s: %s", filename, err)
	}
	return nil
}

// triggerPeriodicJobs triggers jobs at HEAD of the master branch in each repo
// for any files present in the trigger dir.
func (s *TaskScheduler) triggerPeriodicJobs() error {
	triggerDir := path.Join(s.workdir, TRIGGER_DIRNAME)
	triggers, err := findAndParseTriggerFiles(triggerDir)
	if err != nil {
		return err
	}
	if len(triggers) == 0 {
		return nil
	}
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
	for _, trigger := range triggers {
		glog.Infof("Triggering %s tasks", trigger)
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
		if err := s.db.PutJobs(jobs); err != nil {
			return err
		}
		if err := deleteTriggerFile(triggerDir, trigger); err != nil {
			return err
		}
	}
	return nil
}
