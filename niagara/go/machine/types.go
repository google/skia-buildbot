package machine

type Event string

const (
	Booted        = Event("booted")
	StartedTask   = Event("started_task")
	RunningTask   = Event("running_task")
	FinishedTask  = Event("finished_task")
	AbandonedTask = Event("abandoned_task")
	Rebooting     = Event("rebooting")
	Idle          = Event("idle")
)

type Description struct {
	ID          string
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

type Status string

const (
	Ready        = Status("ready")
	Assigned     = Status("assigned")
	Busy         = Status("busy")
	Quarantined  = Status("quarantined")
	Dead         = Status("dead")
	Maintainence = Status("maintainence")
)

const (
	EventAttribute      = "event"
	TaskStatusAttribute = "task_status"
)
