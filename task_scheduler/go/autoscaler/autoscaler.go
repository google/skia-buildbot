package autoscaler

import (
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming/autoscaler"
	"go.skia.org/infra/go/util"
)

// Autoscaler automatically creates and tears down Swarming bots in GCE based
// on demand from the Task Scheduler.
type Autoscaler struct {
	debounce map[string]time.Time
	mtx      sync.RWMutex
	scalers  map[string]*autoscaler.Autoscaler
}

// New returns an Autoscaler instance.
func New(scalers map[string]*autoscaler.Autoscaler) *Autoscaler {
	debounce := make(map[string]time.Time, len(scalers))
	for name, _ := range scalers {
		debounce[name] = time.Time{}
	}
	return &Autoscaler{
		debounce: debounce,
		scalers:  scalers,
	}
}

// Update the Autoscaler's view of the Swarming bots.
func (a *Autoscaler) Update() error {
	group := util.NewNamedErrGroup()
	for name, scaler := range a.scalers {
		// https://golang.org/doc/faq#closures_and_goroutines
		name := name
		scaler := scaler
		group.Go(name, func() error {
			return scaler.Update()
		})
	}
	return group.Wait()
}

// Autoscale the managed bots up or down, if necessary.
func (a *Autoscaler) Autoscale(candidateDimensions [][]string) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	now := time.Now()
	for name, scaler := range a.scalers {
		if now.Sub(a.debounce[name]) < 30*time.Minute {
			continue
		}

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

		// Determine whether we need more bots from the given
		// information for each dimension set:
		//   - Number of free Swarming bots.
		//   - Number of candidate tasks.

		// TODO(borenet): This is just a placeholder. Assume we want as
		// many bots as candidate tasks.
		idle := scaler.Idle()
		running := scaler.NumRunning()
		max := scaler.Total()
		availableToStart := max - running
		if len(idle) < candidates {
			// We want more bots.
			start := len(idle) + candidates
			if start > availableToStart {
				start = availableToStart
			}
			if start == 0 {
				sklog.Warningf("Have %d candidates but no bots to start for dimensions: %v", botDims)
			} else {
				if err := scaler.StartN(start); err != nil {
					return err
				}
				a.debounce[name] = now
			}
		} else if candidates == 0 {
			// If we have no candidate tasks, and utilization is
			// under X%, scale the pool down.
			if scaler.Utilization() < 0.5 {
				stop := running / 2
				if stop > len(idle) {
					stop = len(idle)
				}
				if stop > 0 {
					if err := scaler.Stop(idle[:stop]); err != nil {
						return err
					}
					a.debounce[name] = now
				}
			}
		}
	}
	return nil
}
