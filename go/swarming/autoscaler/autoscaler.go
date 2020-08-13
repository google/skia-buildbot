package autoscaler

/*
	Scales GCE bots in Swarming.

	Assumes that all managed bots have the same dimensions and that bots are
	configured to connect to Swarming automatically. Attempts to play nice
	with Swarming by sending graceful shutdown requests and deleting bots
	after they disconnect.

	Examples:

	as, _ := NewAutoscaler("my-gce-project", "zone-1", "swarming-server.com", "scaler-name", ts, instances)

	// Start 5 bots. This does not block until the bots connect to Swarming.
	// If there are not enough offline bots to start, this may start fewer
	// than the requested number of bots, including starting zero bots, and
	// no error will be returned.
	_ = as.StartN(5)

	// Start a specific set of bots. Returns an error if any bot is already
	// online or has been previously requested to start or stop and has not
	// finished that transition.
	_ = as.Start([]string{"my-bot-1"})

	// Stop 5 bots. Idle bots are prioritized (though our view of the bots
	// is only valid as of the last call to Update(), so the bots we think
	// are idle may actually be busy). If there are not enough online bots
	// to stop, this may stop fewer than the requested number of bots,
	// including zero bots, and no error will be returned.
	_ = as.StopN(5)

	// Update our view of the Swarming bots. This is done automatically in
	// NewAutoscaler and should be done periodically before deciding which
	// bots to start and stop.
	_ = as.Update()

	// List the currently-online bots.
	online := as.ListOnline()
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
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

const (
	AUTOSCALER_DIMENSION = "skia_autoscaler"

	// Possible states for a bot to be in.
	BOT_STATE_ONLINE   BotState = "ONLINE"
	BOT_STATE_OFFLINE  BotState = "OFFLINE"
	BOT_STATE_STARTING BotState = "STARTING"
	BOT_STATE_STOPPING BotState = "STOPPING"
)

var (
	// The auth client passed in to NewAutoscaler should have at least this
	// set of scopes.
	AUTH_SCOPES = append(util.CopyStringSlice(gce.AUTH_SCOPES), swarming.AUTH_SCOPE)

	// Lists the valid bot states.
	VALID_BOT_STATES = []BotState{BOT_STATE_ONLINE, BOT_STATE_OFFLINE, BOT_STATE_STARTING, BOT_STATE_STOPPING}
)

// BotState indicates the current state of a bot, eg. whether it is online or
// offline or transitioning to either of those states.
type BotState string

// Autoscaler manages a set of Swarming bots backed by GCE instances, providing
// summary information about the current state of the bots and methods for
// correctly starting and stopping bots.
// TODO(borenet): This doesn't really "auto" scale anything.
type Autoscaler struct {
	// busy maps instance name to whether the bot was running a Swarming
	// task at the last call to Update().
	busy map[string]bool

	// dimensions are the dimensions of the managed bots, assuming that all
	// bots are the same.
	dimensions util.StringSet

	// Used for starting and stopping GCE instances.
	gceScaler *autoscaler.Autoscaler

	// mtx protects busy, dimensions, and states.
	mtx sync.RWMutex

	// name of this scaler.
	name string

	// Map of instance name to its state at the last call to Update().
	states map[string]BotState

	// API client for interacting with Swarming.
	swarming swarming.ApiClient
}

// NewAutoscaler returns an Autoscaler instance.
func NewAutoscaler(projectId, zone, swarmingServer, name string, ts oauth2.TokenSource, instances []*gce.Instance) (*Autoscaler, error) {
	client := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	return NewAutoscalerWithClient(projectId, zone, swarmingServer, name, client, instances)
}

// NewAutoscalerWithClient returns an Autoscaler instance which uses the given
// http.Client.
func NewAutoscalerWithClient(projectId, zone, swarmingServer, name string, client *http.Client, instances []*gce.Instance) (*Autoscaler, error) {
	s, err := swarming.NewApiClient(client, swarmingServer)
	if err != nil {
		return nil, err
	}
	gceScaler, err := autoscaler.NewAutoscalerWithClient(projectId, zone, client, instances)
	if err != nil {
		return nil, err
	}
	rv := &Autoscaler{
		gceScaler: gceScaler,
		name:      name,
		swarming:  s,
	}
	// Call updateInstanceStatuses() instead of Update() to avoid updating
	// gceScaler twice in a row.
	if err := rv.updateInstanceStatuses(); err != nil {
		return nil, err
	}
	return rv, nil
}

// Update determines whether or not each of the managed instances is
// running. Other methods will operate based on the bot state as of the most
// recent call to Update().
func (a *Autoscaler) Update() error {
	if err := a.gceScaler.Update(); err != nil {
		return err
	}
	return a.updateInstanceStatuses()
}

// updateInstanceStatuses queries Swarming to determine whether the given
// instances are running. Saves the connection statuses for the instances.
func (a *Autoscaler) updateInstanceStatuses() error {
	instances := a.gceScaler.GetNamesOfManagedInstances()
	busy := make(map[string]bool, len(instances))
	var dimensions util.StringSet
	states := make(map[string]BotState, len(instances))
	results, err := a.swarming.ListBots(map[string]string{
		AUTOSCALER_DIMENSION: a.name,
	})
	if err != nil {
		return err
	}
	for _, botInfo := range results {
		state := BOT_STATE_ONLINE
		if botInfo.Deleted {
			state = BOT_STATE_OFFLINE
		} else if botInfo.IsDead {
			state = BOT_STATE_OFFLINE
		}
		dims := util.NewStringSet()
		for _, d := range botInfo.Dimensions {
			for _, val := range d.Value {
				dims[fmt.Sprintf("%s:%s", d.Key, val)] = true
			}
		}
		busy[botInfo.BotId] = state == BOT_STATE_ONLINE && botInfo.TaskId != ""
		states[botInfo.BotId] = state
		if len(dims) > 0 && dimensions == nil {
			dimensions = dims
		}
	}
	for _, instance := range instances {
		if _, ok := states[instance]; !ok {
			// If the bot didn't show up in the search, assume that
			// it's offline and has been deleted or has never
			// connected to Swarming.
			states[instance] = BOT_STATE_OFFLINE
		}
	}
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.busy = busy
	counts := make(map[BotState]int, len(VALID_BOT_STATES))
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
		counts[states[id]]++

		// Update metrics for each bot.
		for _, validState := range VALID_BOT_STATES {
			tags := map[string]string{
				"bot":   id,
				"state": string(validState),
			}
			val := int64(0)
			if states[id] == validState {
				val = int64(1)
			}
			metrics2.GetInt64Metric("swarming_autoscaler_bot_states", tags).Update(val)
		}
	}
	a.states = states
	if len(dimensions) > 0 {
		a.dimensions = dimensions
		sklog.Debugf("Updated dimensions: %v", dimensions.Keys())
	} else if len(a.dimensions) == 0 {
		return fmt.Errorf("Failed to obtain dimensions for this autoscaler; are all bots offline?")
	}

	// Update metrics for numbers of bots in each state.
	for state, count := range counts {
		tags := map[string]string{
			"state": string(state),
		}
		metrics2.GetInt64Metric("swarming_autoscaler_bot_state_counts", tags).Update(int64(count))
	}
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

// Return the list of all managed instances.
func (a *Autoscaler) ListAll() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make([]string, 0, len(a.states))
	for instance, _ := range a.states {
		rv = append(rv, instance)
	}
	sort.Strings(rv)
	return rv
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
	sklog.Debugf("Starting instances:\n%s", strings.Join(instances, "\n"))
	if err := a.gceScaler.Start(instances); err != nil {
		return err
	}
	for _, instance := range instances {
		a.states[instance] = BOT_STATE_STARTING
	}
	return nil
}

// botShutdown performs the necessary tasks to completely shut down a bot, after
// issuing the terminate request to Swarming.
func (a *Autoscaler) botShutdown(name, taskId string) error {
	// Wait for the bot to go offline.
	for {
		sklog.Infof("Waiting for %s to go offline.", name)
		task, err := a.swarming.SwarmingService().Task.Result(taskId).Do()
		if err != nil {
			return fmt.Errorf("Failed to query task status: %s", err)
		}
		if task.State == "COMPLETED" {
			sklog.Infof("%s Done!", name)
			break
		}
		if util.In(task.State, []string{"BOT_DIED", "CANCELED", "EXPIRED", "KILLED", "NO_RESOURCE", "TIMED_OUT"}) {
			return fmt.Errorf("Failed to shut down bot %q with task %q: %s", name, task.TaskId, task.State)
		}
		time.Sleep(time.Minute)
	}
	// Delete the bots from Swarming to avoid alerts.
	sklog.Infof("Deleting bot: %s", name)
	if err := a.swarming.DeleteBots([]string{name}); err != nil {
		return fmt.Errorf("Failed to delete bot %q: %s", name, err)
	}
	// Shut down the GCE instances.
	sklog.Infof("Shutting down GCE instance: %s", name)
	if err := a.gceScaler.Stop([]string{name}); err != nil {
		return fmt.Errorf("Failed to stop instance %q: %s", name, err)
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
	sklog.Debugf("Stopping instances:\n%s", strings.Join(instances, "\n"))
	for _, name := range instances {
		name := name // https://golang.org/doc/faq#closures_and_goroutines
		task, err := a.swarming.GracefullyShutdownBot(name)
		if err != nil {
			return err
		}
		a.states[name] = BOT_STATE_STOPPING

		go func(name, taskId string) {
			completed := false
			defer func() {
				if !completed {
					sklog.Errorf("Did not finish stopping instance %s", name)
				}
			}()
			if err := a.botShutdown(name, taskId); err != nil {
				sklog.Errorf("Failed to stop bot: %s", err)
			}
			completed = true
			sklog.Infof("Successfully shut down %s", name)
		}(name, task.TaskId)
	}
	return nil
}

// StartN starts up to N instances. This does not block until the bots connect
// to Swarming. If there are not enough offline bots to start, this may start
// fewer than the requested number of bots, including starting zero bots, and
// no error will be returned. Bots are selected in alphanumeric order by name.
func (a *Autoscaler) StartN(n int) error {
	offline := a.ListOffline()
	if len(offline) < n {
		n = len(offline)
	}
	return a.Start(offline[:n])
}

// StopN stops up to N instances. Idle bots are prioritized (though our view of
// the bots is only valid as of the last call to Update(), so the bots we think
// are idle may actually be busy). If there are not enough online bots to stop,
// this may stop fewer than the requested number of bots, including zero bots,
// and no error will be returned. Bots are selected in reverse alphanumeric
// order by name, first from the pool of idle bots, then from the general pool
// of online bots.
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
			stop[instance] = true
			if len(stop) == n {
				break
			}
		}
	}
	return a.Stop(stop.Keys())
}
