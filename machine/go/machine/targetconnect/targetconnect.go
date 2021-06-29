// Package targetconnect initiates and maintains a connection from
// a test machine to a switchboard pod. See https://go/skia-switchboard.
package targetconnect

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/machine/store"
	"go.skia.org/infra/machine/go/switchboard"
)

const defaultRetryDelay = time.Second

// RevPortForward is the interface of an object that initiates a reverse
// port-forward into a switchboard pod.
type RevPortForward interface {
	Start(ctx context.Context, podName string, port int) error
}

// Connection that can initiate and maintain a connection from a target machine
// into the switchboard cluster.
type Connection struct {
	switchboard    switchboard.Switchboard
	machineStore   store.Store
	revPortForward RevPortForward

	hostname string
	username string

	meetingPointLiveness metrics2.Liveness
	stepsCounter         metrics2.Counter

	mutex       sync.RWMutex // protects runningTest.
	runningTest bool
}

// New return a new *connection that can initiate and maintain a connection from
// a target machine into the switchboard cluster.
func New(switchboard switchboard.Switchboard, revportforward RevPortForward, machineStore store.Store, hostname, username string) *Connection {
	tags := map[string]string{
		"hostname": hostname,
		"username": username,
	}
	return &Connection{
		switchboard:          switchboard,
		machineStore:         machineStore,
		revPortForward:       revportforward,
		hostname:             hostname,
		username:             username,
		meetingPointLiveness: metrics2.NewLiveness("machine_targetconnect_keep_alive_meeting_point", tags),
		stepsCounter:         metrics2.GetCounter("machine_targetconnect_steps", tags),
	}
}

func (c *Connection) isRunningTest() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.runningTest
}

// Start a connection to a switchboard pod, while keeping Switchboard up to date
// on the status of the connetion.
//
// Start does not return, unless the passed in Context is cancelled.
func (c *Connection) Start(ctx context.Context) error {
	ticker := time.NewTicker(switchboard.MeetingPointKeepAliveDuration)

	// Keep track if this machine is running a test.
	go func() {
		machineCh := c.machineStore.Watch(ctx, c.hostname)
		for {
			select {
			case <-ctx.Done():
				return
			case desc := <-machineCh:
				c.mutex.Lock()
				c.runningTest = desc.RunningSwarmingTask
				c.mutex.Unlock()
			}
		}
	}()

	for {
		c.stepsCounter.Inc(1)
		c.singleStep(ctx, ticker, defaultRetryDelay)
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

// singleStep launches two Go routines, one that creates the reverse port
// forward, and another that keeps Switchboard up to date. If the reverse port
// forward terminates, or the passed in Context is cancelled, then both Go
// routines terminate and the function returns.
func (c *Connection) singleStep(ctx context.Context, ticker *time.Ticker, sleepDurationOnReserveFailure time.Duration) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mp, err := c.switchboard.ReserveMeetingPoint(ctx, c.hostname, c.username)
	if err != nil {
		time.Sleep(sleepDurationOnReserveFailure)
		return
	}

	// We want to wait until both Go routines finish.
	var wg sync.WaitGroup
	wg.Add(2)

	// This Go routine establishes the reverse port forward.
	go func() {
		defer wg.Done()

		// If this Go routine exits, make sure the other Go routine also exits
		// by cancelling the Context. Note that we are only cancelling the
		// context we created at the top of singleStep, not the one passed into
		// Start().
		defer cancel()

		// Only returns on error or if the Context was cancelled.
		err := c.revPortForward.Start(ctx, mp.PodName, mp.Port)
		if err != nil {
			sklog.Warningf("targetconnect revportforward error: %s", err)
		}
		// We use a background context because the passed in context might get
		// cancelled and if it does we still need to call ClearMeetingPoint with
		// an uncancelled Context.
		err = c.switchboard.ClearMeetingPoint(context.Background(), mp)
		if err != nil {
			sklog.Errorf("targetconnect failed to clear meeting point: %s", err)
		}
	}()

	// This Go routine periodically calls KeepAliveMeetingPoint.
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !c.isRunningTest() && !c.switchboard.IsValidPod(ctx, mp.PodName) {
					sklog.Infof("pod is no longer valid, exiting for force reconnect: %q", mp.PodName)
					cancel()
					return
				}
				err := c.switchboard.KeepAliveMeetingPoint(ctx, mp)
				if err != nil {
					sklog.Errorf("targetconnect KeepAliveMeetingPoint failed: %s", err)
				}
				c.meetingPointLiveness.Reset()
			}
		}
	}()

	wg.Wait()
}
