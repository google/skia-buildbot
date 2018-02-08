package types

import "go.skia.org/infra/go/systemd"

// ListResponse is the format of the response from pulld's /_/list endpoint.
type ListResponse struct {
	Hostname string                `json:"hostname"`
	Units    []*systemd.UnitStatus `json:"units"`
}
