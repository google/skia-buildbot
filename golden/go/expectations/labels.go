package expectations

// LabelInt for classifying digests.
type LabelInt int

const (
	// UntriagedInt represents a previously unseen digest.
	UntriagedInt LabelInt = iota // == 0
	// PositiveInt represents a known good digest.
	PositiveInt
	// NegativeInt represents a known bad digest.
	NegativeInt
)

// LabelStr is the string version of LabelInt. Used e.g. to represent digest classifications in JSON.
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
// must match its LabelInt value (Untriaged = 0, etc.).
var AllLabelStr = []LabelStr{UntriagedStr, PositiveStr, NegativeStr}

func (l LabelInt) String() LabelStr {
	return AllLabelStr[l]
}

var labels = map[LabelStr]LabelInt{
	UntriagedStr: UntriagedInt,
	PositiveStr:  PositiveInt,
	NegativeStr:  NegativeInt,
}

// LabelFromString returns the LabelInt corresponding to the given LabelStr, or Untriaged if there is
// no match.
func LabelFromString(s LabelStr) LabelInt {
	if l, ok := labels[s]; ok {
		return l
	}
	return UntriagedInt
}

// ValidLabelStr returns true if the given LabelStr is valid.
func ValidLabelStr(s LabelStr) bool {
	_, ok := labels[s]
	return ok
}
