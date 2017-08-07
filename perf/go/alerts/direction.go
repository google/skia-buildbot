//go:generate stringer -type=Direction
//
package alerts

import "encoding/json"

// Direction a step takes that will cause an alert.
//
// It is an int, but will serialize to/from a string in JSON.
type Direction int

// The values for the Direction enum. Run 'go generate' if you
// add/remove/update these values. You must have 'stringer' installed, i.e.
//
//    go get golang.org/x/tools/cmd/stringer
const (
	BOTH Direction = iota
	UP
	DOWN
	DIRECTION_EOL // End of list.
)

// UnmarshalJSON will decode invalid values as BOTH.
func (a *Direction) UnmarshalJSON(b []byte) error {
	*a = BOTH
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for i := Direction(0); i < DIRECTION_EOL; i++ {
		if Direction(i).String() == s {
			*a = i
			break
		}
	}
	return nil
}

func (a Direction) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}
