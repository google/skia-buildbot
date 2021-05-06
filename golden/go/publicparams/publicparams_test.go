package publicparams

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

const (
	test   = types.PrimaryKeyField
	corpus = types.CorpusField
	model  = "model"
	sdk    = "sdk"

	alphaCorpus = "alpha"
	betaCorpus  = "beta"

	modelOne = "model1"
	modelTwo = "model2"

	sdkOne   = "sdk1"
	sdkTwo   = "sdk2"
	sdkThree = "sdk3"

	testOne = "test1"
	testTwo = "test2"
)

var (
	// These are effectively test cases, representing possible traces that could be compared against
	// the various matchers. "known" means the value matches a constant, "ambivalent" means the value
	// matches a constant (but shouldn't affect any Matcher), "unknown" means the value does not
	// match a constant, "missing" means the key doesn't exist, and "empty" means the value is "".
	// The final _True or _False indicates the expected value of Matches() given the rules in
	// jsonConfig.
	knownCorpus_True          = paramtools.Params{corpus: alphaCorpus, test: testOne}
	knownCorpusModelSDK_True  = paramtools.Params{corpus: betaCorpus, model: modelOne, sdk: sdkOne, test: testOne}
	knownCorpusModelSDK2_True = paramtools.Params{corpus: betaCorpus, model: modelTwo, sdk: sdkTwo, test: testTwo}

	knownCorpus_AmbivalentSDK_True = paramtools.Params{corpus: alphaCorpus, sdk: sdkThree, test: testOne}

	knownCorpusModel_UnknownSDK_False = paramtools.Params{corpus: betaCorpus, model: modelOne, sdk: "unknown", test: testOne}
	knownCorpusSDK_UnknownModel_False = paramtools.Params{corpus: betaCorpus, model: "unknown", sdk: sdkThree, test: testTwo}
	knownCorpus_UnknownModelSDK_False = paramtools.Params{corpus: betaCorpus, model: "unknown", sdk: "unknown", test: testTwo}
	knownModelSDK_UnknownCorpus_False = paramtools.Params{corpus: "unknown", model: modelOne, sdk: sdkTwo, test: testOne}

	knownCorpusModel_MissingSDK_False = paramtools.Params{corpus: betaCorpus, model: modelOne, test: testOne}
	knownCorpus_MissingModelSDK_False = paramtools.Params{corpus: betaCorpus, test: testOne}
	knownModelSDK_MissingCorpus_False = paramtools.Params{model: modelTwo, sdk: sdkTwo, test: testTwo}

	knownCorpusModel_EmptySDK_False = paramtools.Params{corpus: betaCorpus, model: modelOne, sdk: "", test: testOne}
	knownModelSDK_EmptyCorpus_False = paramtools.Params{corpus: "", model: modelOne, sdk: sdkTwo, test: testOne}
)

func TestMatcherFromRules_ValidConfig_RulesAreFollowed(t *testing.T) {
	unittest.SmallTest(t)
	m, err := MatcherFromRules(MatchingRules{
		"alpha": {},
		"beta": {
			"model": {"model1", "model2"},
			"sdk":   {"sdk1", "sdk2", "sdk3"},
		},
	})
	require.NoError(t, err)

	assert.True(t, m.Matches(knownCorpus_True))
	assert.True(t, m.Matches(knownCorpusModelSDK_True))
	assert.True(t, m.Matches(knownCorpusModelSDK2_True))
	assert.True(t, m.Matches(knownCorpus_AmbivalentSDK_True))

	assert.False(t, m.Matches(knownCorpusModel_UnknownSDK_False))
	assert.False(t, m.Matches(knownCorpusSDK_UnknownModel_False))
	assert.False(t, m.Matches(knownCorpus_UnknownModelSDK_False))
	assert.False(t, m.Matches(knownModelSDK_UnknownCorpus_False))
	assert.False(t, m.Matches(knownCorpusModel_MissingSDK_False))
	assert.False(t, m.Matches(knownCorpus_MissingModelSDK_False))
	assert.False(t, m.Matches(knownModelSDK_MissingCorpus_False))
	assert.False(t, m.Matches(knownCorpusModel_EmptySDK_False))
	assert.False(t, m.Matches(knownModelSDK_EmptyCorpus_False))
	assert.False(t, m.Matches(nil))
}

func TestMatcherFromRules_EmptyConfig_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := MatcherFromRules(MatchingRules{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No rules detected")
}

func TestMatcherFromRules_EmptyCorpus_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := MatcherFromRules(MatchingRules{
		"": {},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}
