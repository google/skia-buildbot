// Package switchboard is for keeping track of connections from machines to the
// switchboard pods.
package switchboard

import (
	"errors"
	"time"
)

var ErrMachineNotFound = errors.New("no such machine")

// MeetingPoint has a machine ID and all of the information needed on how to
// connect to that machine via SSH. This is, if a client ran the equivalent of:
//
//    kubectl $PodName port-forward $Port
//
// They could then connect to that machine by running:
//
//    ssh $Username@localhost -p $Port
//
// In practice we won't shell out to kubectl or shh, but instead will make the
// connections via Go code.
type MeetingPoint struct {
	PodName     string
	Port        int
	Username    string
	MachineID   string
	LastUpdated time.Time
}

// Switchboard manages MeetingPoints for all machines.
//
// See also http://go/switchboard-interaction-diagram.
type Switchboard interface {
	// ReserveMeetingPoint is called by bot_config as it starts up and is trying
	// to initiate a connection to the switchboard. The Username is the account
	// name to use when ssh'ing into the machine.
	//
	// Note that Username should be 'chrome-bot' for all instances, but the
	// rack4 RPIs run as root now so they can get access to the USB port from
	// within the k8s container, so at least during the transition we will need
	// to specify the target account name.
	ReserveMeetingPoint(machineID string, Username string) (MeetingPoint, error)

	// ClearMeetingPoint is called by bot_config if it failed to connect to the
	// switchboard or if the machine is shutting down.
	ClearMeetingPoint(meeingPoint MeetingPoint) error

	// GetMeetingPoint returns the information needed to talk to the given
	// machine. This will be called by TaskScheduler once it's decided which
	// machine to run a test on. It may return ErrMachineNotFound if there is no
	// connection for the given machineID.
	GetMeetingPoint(machineID string) (MeetingPoint, error)

	// KeepAliveMeetingPoint is called by bot_config periodically to indicate it
	// is still a valid connection. This updates MeetingPoint.LastUpdated.
	KeepAliveMeetingPoint(meetingPoint MeetingPoint) error

	// AddPod adds a new k8s pod to the list of available pods running in the
	// switchboard cluster. It is called by the programming that runs on startup
	// in each switchboard pod.
	AddPod(PodName string)

	// RemovePod removes a k8s pod from the list of available pods. It is called
	// from each switchboard pod as it shuts down.
	RemovePod(PodName string)

	// ListPods returns a list of all the pods availble to accept connections.
	ListPods() ([]string, error)

	// ListMeetingPoints returns all the active MeetingPoints.
	ListMeetingPoints() ([]MeetingPoint, error)
}
