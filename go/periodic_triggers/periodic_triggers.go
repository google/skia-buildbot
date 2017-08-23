package periodic_triggers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// TRIGGER_DIRNAME is the name of the directory containing files
	// indicating that periodic actions should be triggered. Common practice
	// is to use systemd services to write the files on a timer.
	TRIGGER_DIRNAME = "periodic-trigger"

	// PERIODIC_TRIGGER_MEASUREMENT is the name of the liveness metric for
	// periodic triggers.
	PERIODIC_TRIGGER_MEASUREMENT = "periodic_trigger"

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
		sklog.Errorf("Creating metric %s -- %s", PERIODIC_TRIGGER_MEASUREMENT, name)
		lv = metrics2.NewLiveness(PERIODIC_TRIGGER_MEASUREMENT, map[string]string{
			"trigger": name,
		})
		m.metrics[name] = lv
	}
	lv.ManualReset(now)
	m.LastTriggered[name] = now
}

// Write writes the last-triggered times to a JSON file.
func (m *periodicTriggerMetrics) Write() error {
	return util.WithWriteFile(m.jsonFile, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(m)
	})
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

// deleteTriggerFile removes the given trigger file indicating that the
// triggered function(s) succeeded.
func deleteTriggerFile(triggerDir, basename string) error {
	filename := path.Join(triggerDir, basename)
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("Unable to remove trigger file %s: %s", filename, err)
	}
	return nil
}

// Triggerer is a struct which triggers certain actions on a timer.
type Triggerer struct {
	funcs      map[string][]func() error
	metrics    *periodicTriggerMetrics
	mtx        sync.RWMutex
	triggerDir string
	workdir    string
}

// Return a Triggerer instance.
func NewTriggerer(workdir string) (*Triggerer, error) {
	triggerDir := path.Join(workdir, TRIGGER_DIRNAME)
	if err := os.MkdirAll(triggerDir, os.ModePerm); err != nil {
		return nil, err
	}
	metrics, err := newPeriodicTriggerMetrics(workdir)
	if err != nil {
		return nil, err
	}
	return &Triggerer{
		funcs:      map[string][]func() error{},
		metrics:    metrics,
		mtx:        sync.RWMutex{},
		triggerDir: triggerDir,
		workdir:    workdir,
	}, nil
}

// Register the given function to run at the given trigger.
func (t *Triggerer) Register(trigger string, fn func() error) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.funcs[trigger] = append(t.funcs[trigger], fn)
}

// RunPeriodicTriggers returns the set of triggers which have just fired.
func (t *Triggerer) RunPeriodicTriggers() error {
	triggers, err := findAndParseTriggerFiles(t.triggerDir)
	if err != nil {
		return err
	}

	t.mtx.RLock()
	defer t.mtx.RUnlock()

	// TODO(borenet): Parallelize this?
	allErrs := []error{}
	for _, trigger := range triggers {
		errs := []error{}
		for _, fn := range t.funcs[trigger] {
			if err := fn(); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) == 0 {
			if err := deleteTriggerFile(t.triggerDir, trigger); err != nil {
				allErrs = append(allErrs, err)
			}
			t.metrics.Reset(trigger)
		} else {
			allErrs = append(allErrs, errs...)
		}
	}
	if err := t.metrics.Write(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) > 0 {
		rvMsg := "Encountered errors running periodic triggers:"
		for _, err := range allErrs {
			rvMsg += fmt.Sprintf("\n%s", err)
		}
		return errors.New(rvMsg)
	}
	return nil
}

// Start running periodic triggers in a loop.
func Start(ctx context.Context, workdir string, triggers map[string][]func() error) error {
	t, err := NewTriggerer(workdir)
	if err != nil {
		return err
	}
	for trigger, funcs := range triggers {
		for _, fn := range funcs {
			t.Register(trigger, fn)
		}
	}
	lv := metrics2.NewLiveness("last_successful_periodic_trigger_loop")
	go util.RepeatCtx(time.Minute, ctx, func() {
		if err := t.RunPeriodicTriggers(); err != nil {
			sklog.Errorf("Failed to run periodic triggers: %s", err)
		} else {
			lv.Reset()
		}
	})
	return nil
}
