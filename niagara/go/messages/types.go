package messages

const (
	MachineID = "id"
	Event     = "event"
)

type MachineEvent string

const (
	Booted       = MachineEvent("booted")
	StartedTask  = MachineEvent("started task")
	RunningTask  = MachineEvent("running task")
	FinishedTask = MachineEvent("finished task")
	Rebooting    = MachineEvent("rebooting")
	SittingIdle  = MachineEvent("sitting idle")
)

type MachineState struct {
	Dimensions  map[string][]string
	CurrentTask string
	// TODO(kjlubick)
	//   Device
	//   Temperature
	//   DiskSpace
	//   Uptime
	//   IP
	//   etc
}

type MachineStatus string

const (
	Ready        = MachineStatus("ready")
	Assigned     = MachineStatus("assigned")
	Busy         = MachineStatus("busy")
	Quarantined  = MachineStatus("quarantined")
	Dead         = MachineStatus("dead")
	Maintainence = MachineStatus("maintainence")
)
