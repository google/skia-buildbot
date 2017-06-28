//go:generate stringer -type=AlertConfigState
//
// $ go get golang.org/x/tools/cmd/stringer
package alerts

import "fmt"

// AlertConfigState is the current state of an AlertConfig.
type AlertConfigState int

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
