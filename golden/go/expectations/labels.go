package expectations

// Label for classifying digests.
type Label int

const (
	// Untriaged represents a previously unseen digest.
	Untriaged Label = iota // == 0
	// Positive represents a known good digest.
	Positive
	// Negative represents a known bad digest.
	Negative
)

// LabelStr is the string version of Label. Used e.g. to represent digest classifications in JSON.
type LabelStr string

const (
	// UntriagedStr represents a previously unseen digest. String version of Untriaged.
	UntriagedStr = LabelStr("untriaged")

	// PositiveStr represents a known good digest. String version of Positive.
	PositiveStr = LabelStr("positive")

	// NegativeStr represents a known bad digest. String version of Negative.
	NegativeStr = LabelStr("negative")
)

// AllLabelStr is a list of all possible LabelStr values. The index of each element in this list
// must match its Label value (Untriaged = 0, etc.).
var AllLabelStr = []LabelStr{UntriagedStr, PositiveStr, NegativeStr}

func (l Label) String() LabelStr {
	return AllLabelStr[l]
}

var labels = map[LabelStr]Label{
	UntriagedStr: Untriaged,
	PositiveStr:  Positive,
	NegativeStr:  Negative,
}

// LabelFromString returns the Label corresponding to the given LabelStr, or Untriaged if there is
// no match.
func LabelFromString(s LabelStr) Label {
	if l, ok := labels[s]; ok {
		return l
	}
	return Untriaged
}

// ValidLabelStr returns true if the given LabelStr is valid.
func ValidLabelStr(s LabelStr) bool {
	_, ok := labels[s]
	return ok
}
