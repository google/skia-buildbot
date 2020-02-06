package task

type TaskStatus string

const (
	New     = TaskStatus("new")
	Running = TaskStatus("running")

	Success        = TaskStatus("success")
	Failure        = TaskStatus("failure")
	InfraFailure   = TaskStatus("infra_failure")
	NiagaraFailure = TaskStatus("niagara_failure")

	TimedOut  = TaskStatus("timed_out")
	Cancelled = TaskStatus("cancelled")
)

type MaintenanceTaskType string

const (
	RebootMachine = MaintenanceTaskType("reboot")
	UpdateMachine = MaintenanceTaskType("update")
)
