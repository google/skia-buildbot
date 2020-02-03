package task

type TaskStatus string

const (
	New            = TaskStatus("new")
	Running        = TaskStatus("running")
	Success        = TaskStatus("success")
	Failure        = TaskStatus("failure")
	InfraFailure   = TaskStatus("infra_failure")
	NiagaraFailure = TaskStatus("niagara_failure")
)
