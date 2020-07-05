package alerts

// Direction a step takes that will cause an alert.
//
type Direction string

// The values for the Direction enum. Run 'go generate' if you
// add/remove/update these values. You must have 'stringer' installed, i.e.
//
//    go get golang.org/x/tools/cmd/stringer
const (
	BOTH Direction = "BOTH"
	UP   Direction = "UP"
	DOWN Direction = "DOWN"
)

// AllDirections is a list of all possible Direction values.
var AllDirections = []Direction{
	UP,
	DOWN,
	BOTH,
}
