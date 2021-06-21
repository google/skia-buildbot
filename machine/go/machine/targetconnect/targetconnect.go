// Package targetconnect initiates and maintains a connection from
// a test machine to a switchboard pod. See https://go/skia-switchboard.
package targetconnect

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/switchboard"
)

const retryDelay = time.Second

// revPortForward is the interface of an object that initiates a reverse
// port-forward into a switchboard pod.
type revPortForward interface {
	Start(context.Context) error
}

// Connection that can initiate and maintain a connection from a target machine
// into the switchboard cluster.
type Connection struct {
	switchboard    switchboard.Switchboard
	revPortForward revPortForward

	hostname   string
	username   string
	retryDelay time.Duration
}

// New return a new *connection that can initiate and maintain a connection from
// a target machine into the switchboard cluster.
func New(switchboard switchboard.Switchboard, revportforward revPortForward, hostname, username string) *Connection {
	return &Connection{
		switchboard:    switchboard,
		revPortForward: revportforward,
		hostname:       hostname,
		username:       username,
		retryDelay:     retryDelay,
	}
}

// Start a connection to a switchboard pod, while keeping Switchboard up to date
// on the status of the connetion.
//
// Start does not return, unless the passed in Context is cancelled.
func (c *Connection) Start(ctx context.Context) error {
	ticker := time.NewTicker(switchboard.MeetingPointKeepAliveDuration)

	for {
		c.singleStep(ctx, ticker)
	}
}

func (c *Connection) singleStep(ctx context.Context, ticker *time.Ticker) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mp, err := c.switchboard.ReserveMeetingPoint(ctx, c.hostname, c.username)
	if err != nil {
		time.Sleep(c.retryDelay)
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// If this Go routine exits, make sure the other Go routine also exits.
		defer cancel()
		// Only returns on error or if the Context was cancelled.
		err := c.revPortForward.Start(ctx)
		if err != nil {
			sklog.Warningf("Error from revportforward: %s", err)
		}
		// We use a background context because the passed in context might get
		// cancelled and if it does we still need to call ClearMeetingPoint.
		err = c.switchboard.ClearMeetingPoint(context.Background(), mp)
		if err != nil {
			sklog.Error(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			sklog.Info("started")
			select {
			case <-ctx.Done():
				sklog.Info("cancelled")
				return
			case <-ticker.C:
				err := c.switchboard.KeepAliveMeetingPoint(ctx, mp)
				if err != nil {
					sklog.Errorf("Error calling KeepAliveMeetingPoint: %s", err)
				}
			}
		}
	}()

	wg.Wait()
}
