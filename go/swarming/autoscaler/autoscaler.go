package autoscaler

/*
   Scales GCE bots in Swarming.
*/

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

const (
	MAX_POLL_ATTEMPTS       = 30
	POLL_WAIT_TIME          = 10 * time.Second
	UTILIZATION_TIME_PERIOD = time.Hour

	BOT_STATE_ONLINE   BotState = "ONLINE"
	BOT_STATE_OFFLINE  BotState = "OFFLINE"
	BOT_STATE_STARTING BotState = "STARTING"
	BOT_STATE_STOPPING BotState = "STOPPING"
)

var (
	AUTH_SCOPES = append(util.CopyStringSlice(gce.AUTH_SCOPES), swarming.AUTH_SCOPE)
)

type BotState string

// TODO(borenet): This doesn't really "auto" scale anything.
type Autoscaler struct {
	busy       map[string]bool
	dimensions util.StringSet
	gceScaler  *autoscaler.Autoscaler
	instances  []string
	mtx        sync.RWMutex
	states     map[string]BotState
	swarming   swarming.ApiClient
}

// NewAutoscaler returns an Autoscaler instance.
func NewAutoscaler(projectId, zone, swarmingServer string, client *http.Client, instances []*gce.Instance) (*Autoscaler, error) {
	s, err := swarming.NewApiClient(client, swarmingServer)
	if err != nil {
		return nil, err
	}
	gceScaler, err := autoscaler.NewAutoscaler(projectId, zone, client, instances)
	if err != nil {
		return nil, err
	}
	rv := &Autoscaler{
		gceScaler: gceScaler,
		instances: gceScaler.GetNamesOfManagedInstances(),
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
	return a.updateInstanceStatuses()
}

// updateInstanceStatuses queries Swarming to determine whether the given
// instances are running. Saves the connection statuses for the instances.
func (a *Autoscaler) updateInstanceStatuses() error {
	// TODO(borenet): There must be a more efficient way to do this than to
	// issue one request for each bot.
	busy := make(map[string]bool, len(a.instances))
	var dimensions util.StringSet
	states := make(map[string]BotState, len(a.instances))
	var mtx sync.Mutex
	group := util.NewNamedErrGroup()
	for _, instance := range a.instances {
		instance := instance // https://golang.org/doc/faq#closures_and_goroutines
		// Get the current bot status.
		group.Go(instance, func() error {
			dims := util.StringSet{}
			state := BOT_STATE_ONLINE
			botInfo, err := a.swarming.SwarmingService().Bot.Get(instance).Do()
			if err != nil {
				if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
					// Assume this means that the bot is
					// offline and has never connected to
					// Swarming.
					state = BOT_STATE_OFFLINE
				} else {
					return err
				}
			} else {
				if botInfo.Deleted {
					state = BOT_STATE_OFFLINE
				} else if botInfo.IsDead {
					state = BOT_STATE_OFFLINE
				}
				for _, d := range botInfo.Dimensions {
					for _, val := range d.Value {
						dims[fmt.Sprintf("%s:%s", d.Key, val)] = true
					}
				}
			}
			mtx.Lock()
			defer mtx.Unlock()
			busy[instance] = state == BOT_STATE_ONLINE && botInfo.TaskId != ""
			states[instance] = state
			if len(dims) > 0 && dimensions == nil {
				dimensions = dims
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.busy = busy
	for id, state := range states {
		oldState := a.states[id]
		if state == BOT_STATE_OFFLINE && oldState == BOT_STATE_STARTING {
			// The bot may have formerly been starting. If it's
			// offline we want to maintain the "starting" state.
			states[id] = BOT_STATE_STARTING
		} else if state == BOT_STATE_ONLINE && oldState == BOT_STATE_STOPPING {
			// The bot may still be waiting to go offline (ie. the
			// clean shutdown task may still be pending). Maintain
			// the "stopping" state.
			states[id] = BOT_STATE_STOPPING
		}
	}
	a.states = states
	a.dimensions = dimensions
	return nil
}

// Return the set of dimensions for the bots managed by the Autoscaler.
// Note that this will only be valid if at least one bot is connected, and it
// assumes that dimensions for all managed bots are the same.
func (a *Autoscaler) Dimensions() util.StringSet {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return a.dimensions.Copy()
}

// Return the total number of instances managed by the Autoscaler.
func (a *Autoscaler) Total() int {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	return len(a.states)
}

// Return the list of instances with the given state.
func (a *Autoscaler) ListState(state BotState) []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make([]string, 0, len(a.states))
	for instance, st := range a.states {
		if st == state {
			rv = append(rv, instance)
		}
	}
	sort.Strings(rv)
	return rv
}

// Return the list of online instances as of the last Update().
func (a *Autoscaler) ListOnline() []string {
	return a.ListState(BOT_STATE_ONLINE)
}

// Return the list of offline instances as of the last Update().
func (a *Autoscaler) ListOffline() []string {
	return a.ListState(BOT_STATE_OFFLINE)
}

// Return the list of starting instances as of the last Update().
func (a *Autoscaler) ListStarting() []string {
	return a.ListState(BOT_STATE_STARTING)
}

// Return the list of stopping instances as of the last Update().
func (a *Autoscaler) ListStopping() []string {
	return a.ListState(BOT_STATE_STOPPING)
}

// Return the number of online instances as of the last Update().
func (a *Autoscaler) NumOnline() int {
	return len(a.ListOnline())
}

// Return the number of offline instances as of the last Update().
func (a *Autoscaler) NumOffline() int {
	return len(a.ListOffline())
}

// Return the number of starting instances as of the last Update().
func (a *Autoscaler) NumStarting() int {
	return len(a.ListStarting())
}

// Return the number of stopping instances as of the last Update().
func (a *Autoscaler) NumStopping() int {
	return len(a.ListStopping())
}

// Return the list of online but busy instances as of the last Update().
func (a *Autoscaler) ListBusy() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make([]string, 0, len(a.busy))
	for instance, busy := range a.busy {
		if busy && a.states[instance] == BOT_STATE_ONLINE {
			rv = append(rv, instance)
		}
	}
	sort.Strings(rv)
	return rv
}

// Return the number of online but busy instances as of the last Update().
func (a *Autoscaler) NumBusy() int {
	return len(a.ListBusy())
}

// Return the list of idle instances as of the last Update().
func (a *Autoscaler) ListIdle() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make([]string, 0, len(a.busy))
	for instance, busy := range a.busy {
		if !busy && a.states[instance] == BOT_STATE_ONLINE {
			rv = append(rv, instance)
		}
	}
	sort.Strings(rv)
	return rv
}

// Return the number of online and idle instances as of the last Update().
func (a *Autoscaler) NumIdle() int {
	return len(a.ListIdle())
}

// Start the given instances.
func (a *Autoscaler) Start(instances []string) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	for _, instance := range instances {
		state := a.states[instance]
		if state != BOT_STATE_OFFLINE {
			return fmt.Errorf("Bot %s cannot be started because it is in %q state.", instance, state)
		}
	}
	// Spin up the GCE instances.
	if err := a.gceScaler.Start(instances); err != nil {
		return err
	}
	for _, instance := range instances {
		a.states[instance] = BOT_STATE_STARTING
	}
	return nil
}

// Stop the given instances.
func (a *Autoscaler) Stop(instances []string) error {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	for _, instance := range instances {
		state := a.states[instance]
		if state != BOT_STATE_ONLINE {
			return fmt.Errorf("Bot %s cannot be stopped because it is in %q state.", instance, state)
		}
	}
	// Issue terminate requests to the Swarming bots.
	for _, name := range instances {
		name := name // https://golang.org/doc/faq#closures_and_goroutines
		task, err := a.swarming.GracefullyShutdownBot(name)
		if err != nil {
			return err
		}
		a.states[name] = BOT_STATE_STOPPING

		go func(name, taskId string) {
			// Wait for the bot to go offline.
			for {
				task, err := a.swarming.SwarmingService().Task.Result(task.TaskId).Do()
				if err != nil {
					sklog.Errorf("Failed to query task status: %s", err)
					return
				}
				if task.State == "COMPLETED" {
					break
				}
				if util.In(task.State, []string{"BOT_DIED", "CANCELED", "EXPIRED", "KILLED", "NO_RESOURCE", "TIMED_OUT"}) {
					sklog.Errorf("Failed to shut down bot %q with task %q: %s", name, task.TaskId, task.State)
					return
				}
				time.Sleep(time.Minute)
			}
			// Delete the bots from Swarming to avoid alerts.
			if err := a.swarming.DeleteBots([]string{name}); err != nil {
				sklog.Errorf("Failed to delete bot %q: %s", name, err)
				return
			}
			// Shut down the GCE instances.
			if err := a.gceScaler.Stop([]string{name}); err != nil {
				sklog.Errorf("Failed to stop instance %q: %s", name, err)
				return
			}
		}(name, task.TaskId)
	}
	return nil
}

// StartN starts up to N instances.
func (a *Autoscaler) StartN(n int) error {
	offline := a.ListOffline()
	if len(offline) < n {
		n = len(offline)
	}
	return a.Start(offline[:n])
}

// StopN stops up to N instances.
func (a *Autoscaler) StopN(n int) error {
	// Stop idle instances first, starting in reverse alphanumeric order.
	stop := util.NewStringSet()
	idle := a.ListIdle()
	sort.Sort(sort.Reverse(sort.StringSlice(idle)))
	for _, instance := range idle {
		stop[instance] = true
		if len(stop) == n {
			break
		}
	}
	if len(stop) < n {
		online := a.ListOnline()
		sort.Sort(sort.Reverse(sort.StringSlice(online)))
		for _, instance := range online {
			if !stop[instance] {
				stop[instance] = true
				if len(stop) == n {
					break
				}
			}
		}
	}
	return a.Stop(stop.Keys())
}
