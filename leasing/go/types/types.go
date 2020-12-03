package types

import "time"

type Task struct {
	Requester               string    `json:"requester"`
	OsType                  string    `json:"osType"`
	DeviceType              string    `json:"deviceType"`
	InitialDurationHrs      string    `json:"duration"`
	Created                 time.Time `json:"created"`
	LeaseStartTime          time.Time `json:"leaseStartTime"`
	LeaseEndTime            time.Time `json:"leaseEndTime"`
	Description             string    `json:"description"`
	Done                    bool      `json:"done"`
	WarningSent             bool      `json:"warningSent"`
	EmailThreadingReference string    `json:"emailThreadingReference"`

	TaskIdForIsolates string `json:"taskIdForIsolates"`
	SwarmingPool      string `json:"pool"`
	SwarmingBotId     string `json:"botId"`
	SwarmingServer    string `json:"swarmingServer"`
	SwarmingTaskId    string `json:"swarmingTaskId"`
	SwarmingTaskState string `json:"swarmingTaskState"`

	DatastoreId int64 `json:"datastoreId"`

	// Left for backwards compatibility but no longer used.
	Architecture  string `json:"architecture"`
	SetupDebugger bool   `json:"setupDebugger"`
}

type ExtendTaskRequest struct {
	TaskID      int64 `json:"task"`
	DurationHrs int   `json:"duration"`
}

type ExpireTaskRequest struct {
	TaskID int64 `json:"task"`
}
