package autoscaler

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming/autoscaler"
	"go.skia.org/infra/go/util"
)

const (
	// When there are no tasks to run, we will turn off half of the bots
	// at this interval.
	HALF_LIFE_HOURS = 1

	// We will always leave at least this many bots running, regardless of
	// demand.
	MIN_BOTS = 5
)

var (
	ERR_AUTOSCALE_IN_PROGRESS = errors.New("Autoscale in progress")
)

// Autoscaler automatically creates and tears down Swarming bots in GCE based
// on demand from the Task Scheduler.
type Autoscaler struct {
	inProgress    bool
	inProgressMtx sync.Mutex
	scalers       []*autoscaler.Autoscaler
	targetBots    map[*autoscaler.Autoscaler]int
	targetTime    map[*autoscaler.Autoscaler]time.Time
}

// New returns an Autoscaler instance.
func New(scalers []*autoscaler.Autoscaler, now time.Time) *Autoscaler {
	targetBots := make(map[*autoscaler.Autoscaler]int, len(scalers))
	targetTime := make(map[*autoscaler.Autoscaler]time.Time, len(scalers))
	for _, scaler := range scalers {
		targetBots[scaler] = scaler.NumOnline()
		targetTime[scaler] = now
	}
	return &Autoscaler{
		scalers:    scalers,
		targetBots: targetBots,
		targetTime: targetTime,
	}
}

// Update the Autoscaler's view of the Swarming bots.
func (a *Autoscaler) Update() error {
	group := util.NewNamedErrGroup()
	for i, scaler := range a.scalers {
		// https://golang.org/doc/faq#closures_and_goroutines
		scaler := scaler
		group.Go(fmt.Sprintf("scaler %d", i), func() error {
			return scaler.Update()
		})
	}
	return group.Wait()
}

// Autoscale the managed bots up or down, if necessary. If an autoscale is
// already in progress, returns ERR_AUTOSCALE_IN_PROGRESS.
func (a *Autoscaler) Autoscale(candidateDimensions [][]string, now time.Time) error {
	a.inProgressMtx.Lock()
	defer a.inProgressMtx.Unlock()
	if a.inProgress {
		return ERR_AUTOSCALE_IN_PROGRESS
	}
	a.inProgress = true
	go func() {
		defer func() {
			a.inProgressMtx.Lock()
			defer a.inProgressMtx.Unlock()
			a.inProgress = false
		}()
		if err := a.autoscale(candidateDimensions, now); err != nil {
			sklog.Errorf("Failed to auto-scale: %s", err)
		}
	}()
	return nil
}

// Autoscale the managed bots up or down, if necessary.
func (a *Autoscaler) autoscale(candidateDimensions [][]string, now time.Time) error {
	for idx, scaler := range a.scalers {
		// Find the number of candidates whose dimensions match those
		// of this scaler.
		botDims := scaler.Dimensions()
		candidates := 0
		for _, dims := range candidateDimensions {
			match := true
			for _, dim := range dims {
				if !botDims[dim] {
					match = false
					break
				}
			}
			if match {
				candidates++
			}
		}

		// Determine the target number of bots, based on exponential
		// decay in the case where we have more bots than we need and
		// based on the number of candidates in the case where we have
		// fewer bots than we need.
		elapsedHours := now.Sub(a.targetTime[scaler]).Hours()
		targetF := float64(a.targetBots[scaler]) * math.Pow(0.5, elapsedHours/HALF_LIFE_HOURS)
		target := int(math.Floor(targetF))
		if target < MIN_BOTS {
			target = MIN_BOTS
		}
		needBots := candidates + scaler.NumBusy()
		if needBots > target {
			target = needBots
			maxBots := scaler.Total()
			if target > maxBots {
				target = maxBots
			}
			a.targetBots[scaler] = target
			a.targetTime[scaler] = now
		}

		// Determine how many bots to start or stop based on the target.
		numOnline := scaler.NumOnline()
		numStarting := scaler.NumStarting()
		delta := target - (numOnline + numStarting)

		// Update metrics before scaling.
		allBots := scaler.ListAll()
		tags := map[string]string{
			"bot_range": fmt.Sprintf("%s_%s", allBots[0], allBots[len(allBots)-1]),
			"scaler":    fmt.Sprintf("scaler_%d", idx),
		}
		metrics2.GetInt64Metric("task_scheduler_autoscaler_target", tags).Update(int64(target))
		metrics2.GetInt64Metric("task_scheduler_autoscaler_need_bots", tags).Update(int64(needBots))
		metrics2.GetInt64Metric("task_scheduler_autoscaler_delta", tags).Update(int64(delta))

		// Actually perform the scaling.
		if delta > 0 {
			if err := scaler.StartN(delta); err != nil {
				return err
			}
		} else if delta < 0 {
			if err := scaler.StopN(-delta); err != nil {
				return err
			}
		}
	}
	return nil
}
