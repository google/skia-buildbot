//go:generate stringer -type=AlertConfigState
//
package alerts

import "fmt"

// AlertConfigState is the current state of an AlertConfig.
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

func AlertConfigStateFromName(s string) (AlertConfigState, error) {
	i := AlertConfigState(0)
	for ; i < EOL; i++ {
		if AlertConfigState(i).String() == s {
			return AlertConfigState(i), nil
		}
	}
	return EOL, fmt.Errorf("Enum value not found: %q", s)
}
