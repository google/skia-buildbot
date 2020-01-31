package niagara

const (
	MachineID = "id"
	Event     = "event"
)

type MachineEvent string

const (
	MachineBooted       = MachineEvent("booted")
	MachineStartedTask  = MachineEvent("started_task")
	MachineRunningTask  = MachineEvent("running_task")
	MachineFinishedTask = MachineEvent("finished_task")
	MachineRebooting    = MachineEvent("rebooting")
	MachineIdle         = MachineEvent("idle")
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

type TaskStatus string

const (
	New            = TaskStatus("new")
	Running        = TaskStatus("running")
	Success        = TaskStatus("success")
	Failure        = TaskStatus("failure")
	InfraFailure   = TaskStatus("infra_failure")
	NiagaraFailure = TaskStatus("niagara_failure")
)
