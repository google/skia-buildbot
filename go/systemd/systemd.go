package systemd

import "github.com/skia-dev/go-systemd/dbus"

// UnitStatus describes the status of a single systemd service.
//
// It is serialized to/from JSON between push and pulld.
type UnitStatus struct {
	// Status is the current status of the unit.
	Status *dbus.UnitStatus `json:"status"`

	// Props is the set of unit properties returned from dbus.GetUnitTypeProperties.
	Props map[string]interface{} `json:"props"`
}
