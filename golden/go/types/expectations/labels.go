package expectations

// Label for classifying digests.
type Label int

const (
	UNTRIAGED Label = iota // == 0
	POSITIVE
	NEGATIVE
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
	"untriaged": UNTRIAGED,
	"positive":  POSITIVE,
	"negative":  NEGATIVE,
}

func LabelFromString(s string) Label {
	if l, ok := labels[s]; ok {
		return l
	}
	return UNTRIAGED
}

// ValidLabel returns true if the given label is a valid label string.
func ValidLabel(s string) bool {
	_, ok := labels[s]
	return ok
}
