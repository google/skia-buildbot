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
	Start() error
}

type connection struct {
	switchboard    switchboard.Switchboard
	revportforward revPortForward

	hostname string
	username string
}

func New(switchboard switchboard.Switchboard, revportforward revPortForward, hostname, username string) *connection {
	return &connection{
		switchboard:    switchboard,
		revportforward: revportforward,
		hostname:       hostname,
		username:       username,
	}
}

// Start a connection to a switchboard pod, while keeping Switchboard up to date
// on the status of the connetion.
func (c *connection) Start(ctx context.Context) error {
	ticker := time.NewTicker(switchboard.MeetingPointKeepAliveDuration)
	mp, rpfDoneCh, err := c.connectToPod(ctx)
	if err != nil {
		return err
	}

	for {

		// select on rpfDoneCh, the context being Done, and a ticker to keep switchboard up to date.
	outer:
		for {
			select {
			case <-ctx.Done():
				close(rpfDoneCh)
				return nil
			case <-rpfDoneCh:
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

func (c *connection) connectToPod(ctx context.Context) (switchboard.MeetingPoint, chan bool, error) {
	rpfDoneCh := make(chan bool)
	// Kick off rfp in a Go routine.

	mp, err := c.switchboard.ReserveMeetingPoint(ctx, c.hostname, c.username)
	if err != nil {
		close(rpfDoneCh)
		return mp, nil, err
	}
	go func() {
		err := c.revportforward.Start()
		if err != nil {
			sklog.Warningf("Error from revportforward: %s", err)
		}
		c.switchboard.ClearMeetingPoint(ctx, mp)
		rpfDoneCh <- true
	}()
	return mp, rpfDoneCh, err
}
