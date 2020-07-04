package alerts

// Direction a step takes that will cause an alert.
type Direction int

// The possible values for Direction.
const (
	BOTH Direction = iota
	UP
	DOWN
)

// AllDirections is the list of all valid Directions.
var AllDirections = []Direction{BOTH, UP, DOWN}
