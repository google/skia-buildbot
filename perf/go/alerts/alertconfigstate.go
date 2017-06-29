//go:generate stringer -type=AlertConfigState
//
package alerts

import "encoding/json"

// AlertConfigState is the current state of an AlertConfig.
//
// It is an int, but will serialize to/from a string in JSON.
type AlertConfigState int

// The values for the AlertConfigState enum. Run 'go generate' if you
// add/remove/update these values. You must have 'stringer' installed, i.e.
//
//    go get golang.org/x/tools/cmd/stringer
const (
	ACTIVE AlertConfigState = iota
	DELETED
	EOL // End of list.
)

// UnmarshalJSON will decode invalid values as ACTIVE.
func (a *AlertConfigState) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for i := AlertConfigState(0); i < EOL; i++ {
		if AlertConfigState(i).String() == s {
			*a = i
			break
		}
	}
	return nil
}

func (a AlertConfigState) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}
