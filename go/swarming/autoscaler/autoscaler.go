package autoscaler

/*
   Scales GCE bots in Swarming.
*/

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

const (
	MAX_POLL_ATTEMPTS       = 30
	POLL_WAIT_TIME          = 10 * time.Second
	UTILIZATION_TIME_PERIOD = time.Hour
)

// TODO(borenet): This doesn't really "auto" scale anything.
type Autoscaler struct {
	busy        map[string]bool
	busyTime    map[string]time.Duration
	connected   map[string]bool
	dimensions  util.StringSet
	gceScaler   *autoscaler.Autoscaler
	mtx         sync.RWMutex
	swarming    swarming.ApiClient
	utilization float64
}

// NewAutoscaler returns an Autoscaler instance.
func NewAutoscaler(s swarming.ApiClient, gceScaler *autoscaler.Autoscaler) (*Autoscaler, error) {
	rv := &Autoscaler{
		gceScaler: gceScaler,
		swarming:  s,
	}
	if err := rv.Update(); err != nil {
		return nil, err
	}
	return rv, nil
}

// Update determines whether or not each of the managed instances is
// running.
func (a *Autoscaler) Update() error {
	if err := a.gceScaler.Update(); err != nil {
		return err
	}
	_, err := a.updateInstanceStatuses(a.gceScaler.GetNamesOfManagedInstances())
	return err
}

// updateInstanceStatuses queries Swarming to determine whether the given
// instances are running. Saves the connection statuses for the instances.
func (a *Autoscaler) updateInstanceStatuses(instances []string) (map[string]bool, error) {
	now := time.Now()
	// TODO(borenet): There must be a more efficient way to do this than to
	// issue one request for each bot.
	busy := make(map[string]bool, len(instances))
	busyTimes := make(map[string]time.Duration, len(instances))
	var dimensions util.StringSet
	connected := make(map[string]bool, len(instances))
	tasksStartTime := float64(now.Add(-UTILIZATION_TIME_PERIOD).Unix())
	var mtx sync.Mutex
	group := util.NewNamedErrGroup()
	for _, instance := range instances {
		instance := instance // https://golang.org/doc/faq#closures_and_goroutines
		// Get the current bot status.
		group.Go(instance, func() error {
			isConnected := true
			dims := util.StringSet{}
			botInfo, err := a.swarming.SwarmingService().Bot.Get(instance).Do()
			if err != nil {
				// TODO(borenet): Here we assume that an error
				// means that the bot has been deleted. This is
				// a bad assumption, since this request could
				// fail for any number of reasons.
				isConnected = false
			} else {
				if botInfo.Deleted {
					isConnected = false
				} else if botInfo.IsDead {
					isConnected = false
				}
				dims := util.StringSet{}
				for _, d := range botInfo.Dimensions {
					for _, val := range d.Value {
						dims[fmt.Sprintf("%s:%s", d.Key, val)] = true
					}
				}
			}
			mtx.Lock()
			defer mtx.Unlock()
			busy[instance] = isConnected && botInfo.TaskId != ""
			connected[instance] = isConnected
			if len(dims) > 0 && dimensions == nil {
				dimensions = dims
			}
			return nil
		})
		// Get the utilization of this bot over the last hour.
		group.Go(instance, func() error {
			tasks, err := a.swarming.SwarmingService().Bot.Tasks(instance).Start(tasksStartTime).Do()
			if err != nil {
				return err
			}
			var busyTime time.Duration
			for _, task := range tasks.Items {
				started, err := time.Parse(task.StartedTs, swarming.TIMESTAMP_FORMAT)
				if err != nil {
					return fmt.Errorf("Failed to parse task started timestamp: %s", err)
				}
				var finished time.Time
				if task.CompletedTs == "" {
					finished = now
				} else {
					finished, err = time.Parse(task.CompletedTs, swarming.TIMESTAMP_FORMAT)
					if err != nil {
						return fmt.Errorf("Failed to parse task completion timestamp: %s", err)
					}
				}
				// TODO(borenet): Consider adding a small per-task overhead to account for
				// scheduling time in Swarming. Alternatively, use the task CreatedTs above,
				// since we should have near-zero pending time.
				busyTime += finished.Sub(started)
			}
			mtx.Lock()
			defer mtx.Unlock()
			busyTimes[instance] = busyTime
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	a.mtx.Lock()
	defer a.mtx.Unlock()
	for k, v := range busy {
		a.busy[k] = v
	}
	for k, v := range connected {
		a.connected[k] = v
	}
	for k, v := range busyTimes {
		a.busyTime[k] = v
	}
	a.dimensions = dimensions
	totalBusyTime := time.Duration(0)
	for instance, connected := range a.connected {
		if connected {
			totalBusyTime += a.busyTime[instance]
		}
	}
	a.utilization = float64(totalBusyTime) / float64(time.Duration(len(a.connected))*UTILIZATION_TIME_PERIOD)
	return connected, nil
}

// Return the set of dimensions for the bots managed by the Autoscaler.
// Note that this will only be valid if at least one bot is connected, and it
// assumes that dimensions for all managed bots are the same.
func (a *Autoscaler) Dimensions() util.StringSet {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return a.dimensions.Copy()
}

// Return the number of running instances as of the last Update().
func (a *Autoscaler) NumRunning() int {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := 0
	for _, running := range a.connected {
		if running {
			rv++
		}
	}
	return rv
}

// Return the total number of instances managed by the Autoscaler.
func (a *Autoscaler) Total() int {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return len(a.connected)
}

// Return the list of running instances as of the last Update().
func (a *Autoscaler) RunningInstances() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make([]string, 0, len(a.connected))
	for instance, running := range a.connected {
		if running {
			rv = append(rv, instance)
		}
	}
	sort.Strings(rv)
	return rv
}

// Return the list of stopped instances as of the last Update().
func (a *Autoscaler) StoppedInstances() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make([]string, 0, len(a.connected))
	for instance, running := range a.connected {
		if !running {
			rv = append(rv, instance)
		}
	}
	sort.Strings(rv)
	return rv
}

// Return the list of busy instances as of the last Update().
func (a *Autoscaler) Busy() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make([]string, 0, len(a.busy))
	for instance, busy := range a.busy {
		if busy {
			rv = append(rv, instance)
		}
	}
	sort.Strings(rv)
	return rv
}

// Return the list of idle instances as of the last Update().
func (a *Autoscaler) Idle() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make([]string, 0, len(a.busy))
	for instance, busy := range a.busy {
		if !busy {
			rv = append(rv, instance)
		}
	}
	sort.Strings(rv)
	return rv
}

// Return the percentage of utilization of the connected bots over the last
// UTILIZATION_TIME_PERIOD.
func (a *Autoscaler) Utilization() float64 {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return a.utilization
}

// Wait for the given instances to have the given connection status.
func (a *Autoscaler) waitForStatus(instances []string, wantConnected bool) error {
	remaining := instances
	attempt := 0
	for {
		connected, err := a.updateInstanceStatuses(remaining)
		if err != nil {
			return err
		}
		remaining = make([]string, 0, len(remaining))
		for instance, isConnected := range connected {
			if isConnected != wantConnected {
				remaining = append(remaining, instance)
			}
		}
		if len(remaining) == 0 {
			return nil
		}
		attempt++
		if attempt > MAX_POLL_ATTEMPTS {
			return fmt.Errorf("Instances failed to connect/disconnect to Swarming within %d attempts.", attempt)
		}
		time.Sleep(POLL_WAIT_TIME)
	}
}

// Start the given instances.
func (a *Autoscaler) Start(instances []string) error {
	// Spin up the GCE instances.
	if err := a.gceScaler.Start(instances); err != nil {
		return err
	}
	// Wait for all of the bots to connect.
	return a.waitForStatus(instances, true)
}

// Stop the given instances.
func (a *Autoscaler) Stop(instances []string) error {
	// Issue terminate requests to the Swarming bots.
	group := util.NewNamedErrGroup()
	for _, name := range instances {
		name := name // https://golang.org/doc/faq#closures_and_goroutines
		group.Go(name, func() error {
			_, err := a.swarming.GracefullyShutdownBot(name)
			return err
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	// Wait for all of the bots to disconnect.
	if err := a.waitForStatus(instances, false); err != nil {
		return err
	}
	// Delete the bots from Swarming to avoid alerts.
	if err := a.swarming.DeleteBots(instances); err != nil {
		return err
	}
	// Shut down the GCE instances.
	if err := a.gceScaler.Stop(instances); err != nil {
		return err
	}
	return nil
}

// StartN starts up to N instances.
func (a *Autoscaler) StartN(n int) error {
	stopped := a.StoppedInstances()
	if len(stopped) < n {
		n = len(stopped)
	}
	return a.Start(stopped[:n])
}

// StopN stops up to N instances.
func (a *Autoscaler) StopN(n int) error {
	running := a.RunningInstances()
	if len(running) < n {
		n = len(running)
	}
	// TODO(borenet): Is there a reason to be smart about which instances
	// we stop, eg. longest running instances?
	return a.Stop(running[:n])
}
