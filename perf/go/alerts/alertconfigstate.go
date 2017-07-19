//go:generate stringer -type=ConfigState
//
package alerts

import "encoding/json"

// ConfigState is the current state of an alerts.Config.
//
// It is an int, but will serialize to/from a string in JSON.
type ConfigState int

// The values for the AlertConfigState enum. Run 'go generate' if you
// add/remove/update these values. You must have 'stringer' installed, i.e.
//
//    go get golang.org/x/tools/cmd/stringer
const (
	ACTIVE ConfigState = iota
	DELETED
	EOL // End of list.
)

// UnmarshalJSON will decode invalid values as ACTIVE.
func (a *ConfigState) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for i := ConfigState(0); i < EOL; i++ {
		if ConfigState(i).String() == s {
			*a = i
			break
		}
	}
	return nil
}

func (a ConfigState) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}
