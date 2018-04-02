package types

import "go.skia.org/infra/go/systemd"

// ListResponse is the format of the response from pulld's /_/list endpoint.
type ListResponse struct {
	Hostname string                `json:"hostname"`
	Units    []*systemd.UnitStatus `json:"units"`
}

type Action string

// Action constants.
const (
	Start   Action = "start"
	Stop    Action = "stop"
	Restart Action = "restart"
	Pull    Action = "pull"
)

// Command represents the commands that can be sent to a pulld instance.
type Command struct {
	Action  Action
	Service string // Name of the service the action applies to, if appropriate.
}
