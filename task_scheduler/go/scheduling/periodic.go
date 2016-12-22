package scheduling

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	// TRIGGER_DIRNAME is the name of the directory containing files
	// indicating that periodic jobs should be triggered. These files are
	// created by the systemd task-scheduler-trigger-*.service.
	TRIGGER_DIRNAME = "periodic-job-trigger"

	// PERIODIC_TRIGGER_MEASUREMENT is the name of the liveness metric for
	// periodic triggers.
	PERIODIC_TRIGGER_MEASUREMENT = "task-scheduler-periodic-trigger"

	// LAST_TRIGGERED_JSON_FILE is the name of a JSON file containing the
	// last-triggered time for each known trigger.
	LAST_TRIGGERED_JSON_FILE = "last-triggered.json"
)

// periodicTriggerMetrics tracks liveness metrics for various periodic triggers.
type periodicTriggerMetrics struct {
	jsonFile      string
	LastTriggered map[string]time.Time `json:"last_triggered"`
	metrics       map[string]metrics2.Liveness
}

// newPeriodicTriggerMetrics returns a periodicTriggerMetrics instance,
// pre-filled with data from a file.
func newPeriodicTriggerMetrics(workdir string) (*periodicTriggerMetrics, error) {
	var rv periodicTriggerMetrics
	jsonFile := path.Join(workdir, LAST_TRIGGERED_JSON_FILE)
	f, err := os.Open(jsonFile)
	if err == nil {
		defer util.Close(f)
		if err := json.NewDecoder(f).Decode(&rv); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	} else {
		rv.LastTriggered = map[string]time.Time{}
	}
	rv.jsonFile = jsonFile
	rv.metrics = make(map[string]metrics2.Liveness, len(rv.LastTriggered))
	for trigger, last := range rv.LastTriggered {
		lv := metrics2.NewLiveness(PERIODIC_TRIGGER_MEASUREMENT, map[string]string{
			"trigger": trigger,
		})
		lv.ManualReset(last)
		rv.metrics[trigger] = lv
	}
	return &rv, nil
}

// Reset resets the given trigger metric.
func (m *periodicTriggerMetrics) Reset(name string) {
	now := time.Now()
	lv, ok := m.metrics[name]
	if !ok {
		lv = metrics2.NewLiveness(PERIODIC_TRIGGER_MEASUREMENT, map[string]string{
			"trigger": name,
		})
		m.metrics[name] = lv
	}
	lv.ManualReset(now)
	m.LastTriggered[name] = now
}

// Write writes the last-triggered times to a JSON file.
func (m *periodicTriggerMetrics) Write() (rv error) {
	f, err := os.Create(m.jsonFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			if rv == nil {
				rv = err
			} else {
				rv = fmt.Errorf("%s; failed to close file: %s", rv, err)
			}
		}
	}()
	return json.NewEncoder(f).Encode(m)
}

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
		if err := s.db.PutJobs(jobs); err != nil {
			return err
		}
		if err := deleteTriggerFile(triggerDir, trigger); err != nil {
			return err
		}
		s.triggerMetrics.Reset(trigger)
	}
	return s.triggerMetrics.Write()
}
