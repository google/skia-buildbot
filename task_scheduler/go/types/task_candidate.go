package types

import "go.skia.org/infra/task_scheduler/go/specs"

// TaskCandidate is a struct used for determining which tasks to schedule.
type TaskCandidate struct {
	Attempt        int      `json:"attempt"`
	Blamelist      []string `json:"blamelist"`
	CasInput       string   `json:"casInput"`
	CasDigests     []string `json:"casDigests"`
	CasUsesIsolate bool     `json:"casUsesIsolate"`
	ParentTaskIds  []string `json:"parentTaskIds"`
	RetryOf        string   `json:"retryOf"`
	Score          float64  `json:"score"`
	StealingFromId string   `json:"stealingFromId"`
	TaskKey
	TaskSpec *specs.TaskSpec `json:"taskSpec"`
	WantedBy []string
}
