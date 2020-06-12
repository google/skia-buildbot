package publicparams

import (
	"github.com/flynn/json5"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/types"
)

// Matcher is the interface for determining if a trace's params fit a certain set of conditions.
type Matcher interface {
	Matches(traceParams paramtools.Params) bool
}

type corpusName string
type keyField string
type value string

type matchingRules map[corpusName]map[keyField][]value

type paramMatcher struct {
	Rules matchingRules
}

// Matches implements the Matcher interface following a set of rules set when constructed.
// If a param is not in a set of required params or does not match the list of known corpora,
// this will return false.
func (m paramMatcher) Matches(traceParams paramtools.Params) bool {
	if len(traceParams) == 0 {
		return false
	}
	corpus, ok := traceParams[types.CorpusField]
	if !ok {
		return false
	}
	requiredKeys, corpusOK := m.Rules[corpusName(corpus)]
	if !corpusOK {
		return false
	}
	for key, values := range requiredKeys {
		traceValue, traceHasKey := traceParams[string(key)]
		if !traceHasKey {
			return false
		}
		matchedValue := false
		for _, v := range values {
			if string(v) == traceValue {
				matchedValue = true
				break
			}
		}
		if !matchedValue {
			return false
		}
	}
	return true
}

// MatcherFromJSON creates a param matcher from a JSON map. The top level keys in this map are
// the publicly viewable corpora. The values for those corpora is another map of required keys
// and their corresponding allowed values. If a trace belonging to a given corpus has all of the
// required keys and the corresponding values are in the list of allowed values, then it will
// be publicly visible. Otherwise, it will not be. See publicparams_test.go for a concrete example.
func MatcherFromJSON(jsonBytes []byte) (*paramMatcher, error) {
	var rules matchingRules
	if err := json5.Unmarshal(jsonBytes, &rules); err != nil {
		return nil, skerr.Wrap(err)
	}

	if len(rules) == 0 {
		return nil, skerr.Fmt("No rules detected.")
	}

	if _, hasEmptyCorpus := rules[""]; hasEmptyCorpus {
		return nil, skerr.Fmt("Cannot contain empty corpus")
	}

	return &paramMatcher{Rules: rules}, nil
}

// Make sure paramMatcher implements the Matcher interface.
var _ Matcher = (*paramMatcher)(nil)
