package machine

type Event string

const (
	Booted       = Event("booted")
	Idle         = Event("idle")
	StartedTask  = Event("started_task")
	RunningTask  = Event("running_task")
	FinishedTask = Event("finished_task")
	Rebooting    = Event("rebooting")
)

type Description struct {
	ID         string
	Dimensions map[string][]string
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
	Ready       = Status("ready")
	Assigned    = Status("assigned")
	Busy        = Status("busy")
	Quarantined = Status("quarantined")
	Dead        = Status("dead")
	Maintenance = Status("maintenance")
)

const (
	EventAttribute       = "event"
	TaskStatusAttribute  = "task_status"
	CurrentTaskAttribute = "current_task"
)
