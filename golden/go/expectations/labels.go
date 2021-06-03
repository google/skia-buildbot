package expectations

// Label represents a digest classification.
type Label string

const (
	// Untriaged represents a previously unseen digest.
	Untriaged = Label("untriaged")

	// Positive represents a known good digest
	Positive = Label("positive")

	// Negative represents a known bad digest.
	Negative = Label("negative")
)

// AllLabel is a list of all possible Label values. The index of each element in this list
// must match its LabelInt value (Untriaged = 0, etc.).
var AllLabel = []Label{Untriaged, Positive, Negative}

// ValidLabel returns true if the given Label is valid.
func ValidLabel(s Label) bool {
	for _, label := range AllLabel {
		if label == s {
			return true
		}
	}
	return false
}
