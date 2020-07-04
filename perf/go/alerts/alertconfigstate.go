package alerts

// ConfigState is the current state of an alerts.Config.
type ConfigState int

// The values for the AlertConfigState enum.
const (
	ACTIVE ConfigState = iota
	DELETED
	EOL // End of list.
)

// AllConfigState is a list of all valid ConfigState values.
var AllConfigState = []ConfigState{
	ACTIVE,
	DELETED,
}
