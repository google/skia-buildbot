package alerts

// ConfigState is the current state of an alerts.Config.
//
type ConfigState string

// The values for the AlertConfigState enum. Run 'go generate' if you
// add/remove/update these values. You must have 'stringer' installed, i.e.
//
//    go get golang.org/x/tools/cmd/stringer
const (
	ACTIVE  ConfigState = "ACTIVE"
	DELETED ConfigState = "DELETED"
)

// AllConfigState is a list of all possible ConfigState values.
var AllConfigState = []ConfigState{
	ACTIVE,
	DELETED,
}

// ConfigStateToInt converts the string ConfigState into an int, which it used
// to be, used only when storing Alerts.
func ConfigStateToInt(c ConfigState) int {
	if c == DELETED {
		return 1
	}
	return 0
}
