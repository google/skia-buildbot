package expectations

// LabelInt is the integer version of Label, used as the storage format in fs_expstore.go and in
// goldctl.
//
// TODO(skbug.com/10522): Make this private to fs_expstore.go once we migrate goldctl to use Label.
type LabelInt int

const (
	// UntriagedInt represents a previously unseen digest. Int version of Untriaged.
	UntriagedInt LabelInt = iota // == 0
	// PositiveInt represents a known good digest. Int version of Positive.
	PositiveInt
	// NegativeInt represents a known bad digest. Int version of Negative.
	NegativeInt
)

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

func (l LabelInt) String() Label {
	return AllLabel[l]
}

var labels = map[Label]LabelInt{
	Untriaged: UntriagedInt,
	Positive:  PositiveInt,
	Negative:  NegativeInt,
}

// LabelIntFromString returns the LabelInt corresponding to the given Label, or Untriaged if there is
// no match.
func LabelIntFromString(s Label) LabelInt {
	if l, ok := labels[s]; ok {
		return l
	}
	return UntriagedInt
}

// ValidLabel returns true if the given Label is valid.
func ValidLabel(s Label) bool {
	_, ok := labels[s]
	return ok
}
