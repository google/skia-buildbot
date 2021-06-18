// Package targetconnect initiates and maintains a connection from
// a test machine to a switchboard pod. See https://go/skia-switchboard.
package targetconnect

import (
	"context"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/switchboard"
)

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

	hostname string
	username string
}

// New return a new *connection that can initiate and maintain a connection from
// a target machine into the switchboard cluster.
func New(switchboard switchboard.Switchboard, revportforward revPortForward, hostname, username string) *Connection {
	return &Connection{
		switchboard:    switchboard,
		revPortForward: revportforward,
		hostname:       hostname,
		username:       username,
	}
}

// Start a connection to a switchboard pod, while keeping Switchboard up to date
// on the status of the connetion.
//
// Start will exit if the passed in Context is cancelled.
func (c *Connection) Start(ctx context.Context) error {
	ticker := time.NewTicker(switchboard.MeetingPointKeepAliveDuration)
	mp, rpfDoneCh, err := c.connectToPod(ctx)
	if err != nil {
		return err
	}

	for {
	outer:
		for {
			select {
			case <-ctx.Done():
				close(rpfDoneCh)
				return nil
			case <-rpfDoneCh:
				// Causes us to exit the inner for loop and to initiate a fresh
				// connection to the cluster.
				close(rpfDoneCh)
				break outer
			case <-ticker.C:
				c.switchboard.KeepAliveMeetingPoint(ctx, mp)
			}
		}
		mp, rpfDoneCh, err = c.connectToPod(ctx)
		if err != nil {
			return err
		}

	}
}

func (c *Connection) connectToPod(ctx context.Context) (switchboard.MeetingPoint, chan bool, error) {
	rpfDoneCh := make(chan bool)
	// Kick off rfp in a Go routine.

	mp, err := c.switchboard.ReserveMeetingPoint(ctx, c.hostname, c.username)
	if err != nil {
		close(rpfDoneCh)
		return mp, nil, err
	}
	go func() {
		err := c.revPortForward.Start(ctx)
		if err != nil {
			sklog.Warningf("Error from revportforward: %s", err)
		}
		// We use a background context because the passed in context might get
		// cancelled and if it does we still need to call ClearMeetingPoint.
		c.switchboard.ClearMeetingPoint(context.Background(), mp)
		rpfDoneCh <- true
	}()
	return mp, rpfDoneCh, err
}
