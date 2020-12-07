package types

import "time"

// Task represents a leasing task.
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

// PoolDetails contains map of OS types to the count of bots in each OS type.
// Eg: {"Android": 84, "ChromOS": 4, ...}
// Also contains map of OS types to Device types to the count of bots in each
// device type. Eg: {"Android" : {"P30": 4, "Pixel4XL": 8, ...}}
type PoolDetails struct {
	OsTypes         map[string]int            `json:"os_types"`
	OsToDeviceTypes map[string]map[string]int `json:"os_to_device_types"`
}

// ExtendTaskRequest is the request used by the extend task endpoint.
type ExtendTaskRequest struct {
	TaskID      int64 `json:"task"`
	DurationHrs int   `json:"duration"`
}

// ExpireTaskRequest is the request used by the expire task endpoint.
type ExpireTaskRequest struct {
	TaskID int64 `json:"task"`
}
