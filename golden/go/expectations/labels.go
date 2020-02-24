package expectations

// Label for classifying digests.
type Label int

const (
	// Untriaged represents a previously unseen digest
	Untriaged Label = iota // == 0
	// Positive represents a known good digest
	Positive
	// Negative represents a known bad digest
	Negative
)

// String representation for Labels. The order must match order above.
var labelStringRepresentation = []string{
	"untriaged",
	"positive",
	"negative",
}

func (l Label) String() string {
	return labelStringRepresentation[l]
}

var labels = map[string]Label{
	"untriaged": Untriaged,
	"positive":  Positive,
	"negative":  Negative,
}

// LabelFromString returns the Label corresponding to the serialized string or Untriaged
// if there is no match.
func LabelFromString(s string) Label {
	if l, ok := labels[s]; ok {
		return l
	}
	return Untriaged
}

// ValidLabel returns true if the given label is a valid label string.
func ValidLabel(s string) bool {
	_, ok := labels[s]
	return ok
}
