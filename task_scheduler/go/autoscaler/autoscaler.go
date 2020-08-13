package autoscaler

import (
	"fmt"
	"math"
	"net/http"
	"sync"
	"time"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming/autoscaler"
)

const (
	// When there are no tasks to run, we will turn off half of the bots
	// at this interval.
	HALF_LIFE_HOURS = 1

	// We will always leave at least this many bots running, regardless of
	// demand.
	MIN_BOTS = 5
)

// Autoscaler automatically creates and tears down Swarming bots in GCE based
// on demand from the Task Scheduler.
type Autoscaler struct {
	mtx        sync.Mutex // Held for the entire Autoscale.
	name       string
	scaler     *autoscaler.Autoscaler
	targetBots int
	targetTime time.Time
}

// New returns an Autoscaler instance.
func New(project, zone, swarmingServer, name string, httpClient *http.Client, instances []*gce.Instance) (*Autoscaler, error) {
	scaler, err := autoscaler.NewAutoscalerWithClient(project, zone, swarmingServer, name, httpClient, instances)
	if err != nil {
		return nil, err
	}
	return &Autoscaler{
		name:       name,
		scaler:     scaler,
		targetBots: scaler.NumOnline(),
		targetTime: time.Now(),
	}, nil
}

// Autoscale the managed bots up or down, if necessary.
func (a *Autoscaler) Autoscale(candidateCount int) error {
	return a.autoscale(candidateCount, time.Now())
}

// Helper function for Autoscale; used for testing.
func (a *Autoscaler) autoscale(candidateCount int, now time.Time) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	// Update our view of the Swarming bots.
	if err := a.scaler.Update(); err != nil {
		return fmt.Errorf("Failed to update swarming bot scaler: %s", err)
	}

	// Determine the target number of bots, based on exponential
	// decay in the case where we have more bots than we need and
	// based on the number of candidates in the case where we have
	// fewer bots than we need.
	elapsedHours := now.Sub(a.targetTime).Hours()
	targetF := float64(a.targetBots) * math.Pow(0.5, elapsedHours/HALF_LIFE_HOURS)
	target := int(math.Floor(targetF))
	if target < MIN_BOTS {
		target = MIN_BOTS
	}
	needBots := candidateCount + a.scaler.NumBusy()
	if needBots > target {
		target = needBots
		maxBots := a.scaler.Total()
		if target > maxBots {
			target = maxBots
		}
		a.targetBots = target
		a.targetTime = now
	}

	// Determine how many bots to start or stop based on the target.
	numOnline := a.scaler.NumOnline()
	numStarting := a.scaler.NumStarting()
	delta := target - (numOnline + numStarting)

	// Update metrics before scaling.
	allBots := a.scaler.ListAll()
	tags := map[string]string{
		"bot_range": fmt.Sprintf("%s_%s", allBots[0], allBots[len(allBots)-1]),
		"scaler":    a.name,
	}
	metrics2.GetInt64Metric("task_scheduler_autoscaler_target", tags).Update(int64(target))
	metrics2.GetInt64Metric("task_scheduler_autoscaler_need_bots", tags).Update(int64(needBots))
	metrics2.GetInt64Metric("task_scheduler_autoscaler_delta", tags).Update(int64(delta))

	// Actually perform the scaling.
	sklog.Infof("delta = %d", delta)
	if delta > 0 {
		if err := a.scaler.StartN(delta); err != nil {
			return err
		}
	} else if delta < 0 {
		if err := a.scaler.StopN(-delta); err != nil {
			return err
		}
	}
	return nil
}

// Return the dimensions of the bots managed by this Autoscaler.
func (a *Autoscaler) Dimensions() map[string]bool {
	return a.scaler.Dimensions()
}
