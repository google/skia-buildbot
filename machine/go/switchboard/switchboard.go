// Package switchboard is for keeping track of connections from machines to the
// switchboard pods. See the design doc: http://go/skia-switchboard.
package switchboard

import (
	"context"
	"errors"
	"time"
)

// ErrMachineNotFound is returned when a given machineID is not found.
var ErrMachineNotFound = errors.New("no such machine")

// ErrNoPodsFound is returned when no pods have been registered.
var ErrNoPodsFound = errors.New("no pods found")

// Pod describes a single pod in the skia-switchboard cluster.
type Pod struct {
	// Name is the pod name in the kubernetes cluster.
	Name string

	// LastUpdated is updated every time Switchboard.KeepAlivePod is
	// called, which will be done by switch-pod-monitor.
	//
	// The machines server will have a background process that monitors for
	// expired Pods and removes them.
	LastUpdated time.Time
}

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
	// PodName is the name of a pod in the skia-switchboard cluster that the machine
	// identified by MachineID is connected to.
	PodName string

	// Port is the TCP port that is used in PodName in the skia-switchboard
	// cluster that the machine identified by MachineID is connected to.
	Port int

	// Username is the account that should be used when SSH'ing into a machine,
	// which is normally 'chrome-bot', but needs to be 'root' in the case of
	// RPis on rack4.
	Username string

	// MachineID is the domain name of the machine, e.g. 'skia-rpi-001'.
	MachineID string

	// LastUpdated is updated every time Switchboard.KeepAliveMeetingPoint is
	// called, which will be done by bot_config,since bot_config is in charge of
	// initiating and keeping the connection to the switchboard cluster
	// connected.
	//
	// The machines server will have a background process that monitors for
	// expired MeetingPoints and removes them.
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
	ReserveMeetingPoint(ctx context.Context, machineID string, username string) (MeetingPoint, error)

	// ClearMeetingPoint is called by bot_config if it failed to connect to the
	// switchboard or if the machine is shutting down, i.e. bot_config
	// determines it is not able to handle incoming connections.
	ClearMeetingPoint(ctx context.Context, meeingPoint MeetingPoint) error

	// GetMeetingPoint returns the information needed to talk to the given
	// machine. This will be called by TaskScheduler once it's decided which
	// machine to run a test on. It may return ErrMachineNotFound if there is no
	// connection for the given machineID.
	GetMeetingPoint(ctx context.Context, machineID string) (MeetingPoint, error)

	// KeepAliveMeetingPoint is called by bot_config periodically to indicate it
	// is still a valid connection. This updates MeetingPoint.LastUpdated.
	KeepAliveMeetingPoint(ctx context.Context, meetingPoint MeetingPoint) error

	// AddPod adds a new k8s pod to the list of available pods running in the
	// switchboard cluster. It is called by the programming that runs on startup
	// in each switchboard pod.
	AddPod(ctx context.Context, podName string) error

	// KeepAlivePod is called by a pod periodically to indicate it
	// is still a valid connection.
	KeepAlivePod(ctx context.Context, podName string) error

	// RemovePod removes a k8s pod from the list of available pods. It is called
	// from each switchboard pod as it shuts down.
	RemovePod(ctx context.Context, podName string) error

	// ListPods returns a list of all the pods availble to accept connections.
	// This will be used in the machines UI.
	ListPods(ctx context.Context) ([]Pod, error)

	// ListMeetingPoints returns all the active MeetingPoints. This will be used
	// in the machines UI.
	ListMeetingPoints(ctx context.Context) ([]MeetingPoint, error)
}
