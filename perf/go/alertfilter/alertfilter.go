//go:generate stringer -type=AlertFilter
//
package alertfilter

import "encoding/json"

// It is an int, but will serialize to/from a string in JSON.
type AlertFilter int

// The values for the AlertFilter enum. Run 'go generate' if you
// add/remove/update these values. You must have 'stringer' installed, i.e.
//
//    go get golang.org/x/tools/cmd/stringer
const (
	ALL AlertFilter = iota
	OWNER
	EOL // End of list.
)

// UnmarshalJSON will decode invalid values as ACTIVE.
func (a *AlertFilter) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for i := AlertFilter(0); i < EOL; i++ {
		if AlertFilter(i).String() == s {
			*a = i
			break
		}
	}
	return nil
}

func (a AlertFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}
