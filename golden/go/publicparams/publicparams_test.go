package publicparams

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/types"
)

const (
	test   = types.PrimaryKeyField
	corpus = types.CorpusField
	model  = "model"
	sdk    = "sdk"

	alphaCorpus = "alpha"
	betaCorpus  = "beta"

	gammaModel = "gamma"
	deltaModel = "delta"

	epsilonSDK = "epsilon"
	zetaSDK    = "zeta"
	thetaSDK   = "theta"

	iotaTest  = "iota"
	kappaTest = "kappa"

	// jsonConfig is valid JSON that can be used to create a Matcher. It uses the above string
	// constants in a way consistent with real-world data.
	jsonConfig = `
{
  "alpha": {},
  "beta": {
    "model": ["gamma", "delta"],
    "sdk": ["epsilon", "zeta", "theta"]
  }
}`
)

var (
	// These are effectively test cases, representing possible traces that could be compared against
	// the various matchers. "known" means the value matches a constant, "ambivalent" means the value
	// matches a constant (but shouldn't affect any Matcher), "unknown" means the value does not
	// match a constant, "missing" means the key doesn't exist, and "empty" means the value is "".
	knownCorpus          = paramtools.Params{corpus: alphaCorpus, test: iotaTest}
	knownCorpusModelSDK  = paramtools.Params{corpus: betaCorpus, model: gammaModel, sdk: epsilonSDK, test: iotaTest}
	knownCorpusModelSDK2 = paramtools.Params{corpus: betaCorpus, model: deltaModel, sdk: zetaSDK, test: kappaTest}

	knownCorpus_AmbivalentSDK = paramtools.Params{corpus: alphaCorpus, sdk: thetaSDK, test: iotaTest}

	knownCorpusModel_UnknownSDK = paramtools.Params{corpus: betaCorpus, model: gammaModel, sdk: "unknown", test: iotaTest}
	knownCorpusSDK_UnknownModel = paramtools.Params{corpus: betaCorpus, model: "unknown", sdk: thetaSDK, test: kappaTest}
	knownCorpus_UnknownModelSDK = paramtools.Params{corpus: betaCorpus, model: "unknown", sdk: "unknown", test: kappaTest}
	knownModelSDK_UnknownCorpus = paramtools.Params{corpus: "unknown", model: gammaModel, sdk: zetaSDK, test: iotaTest}

	knownCorpusModel_MissingSDK = paramtools.Params{corpus: betaCorpus, model: gammaModel, test: iotaTest}
	knownCorpus_MissingModelSDK = paramtools.Params{corpus: betaCorpus, test: iotaTest}
	knownModelSDK_MissingCorpus = paramtools.Params{model: deltaModel, sdk: zetaSDK, test: kappaTest}

	knownCorpusModel_EmptySDK = paramtools.Params{corpus: betaCorpus, model: gammaModel, sdk: "", test: iotaTest}
)

func TestEverything_AlwaysTrue(t *testing.T) {
	ev := Everything()
	assert.True(t, ev.Matches(knownCorpus))
	assert.True(t, ev.Matches(knownCorpusModelSDK))
	assert.True(t, ev.Matches(knownCorpusModelSDK2))
	assert.True(t, ev.Matches(knownCorpus_AmbivalentSDK))
	assert.True(t, ev.Matches(knownCorpusModel_UnknownSDK))
	assert.True(t, ev.Matches(knownCorpusSDK_UnknownModel))
	assert.True(t, ev.Matches(knownCorpus_UnknownModelSDK))
	assert.True(t, ev.Matches(knownModelSDK_UnknownCorpus))
	assert.True(t, ev.Matches(knownCorpusModel_MissingSDK))
	assert.True(t, ev.Matches(knownCorpus_MissingModelSDK))
	assert.True(t, ev.Matches(knownModelSDK_MissingCorpus))
	assert.True(t, ev.Matches(knownCorpusModel_EmptySDK))
	assert.True(t, ev.Matches(nil))
}

func TestMatcherFromJSON_ValidConfig_RulesAreFollowed(t *testing.T) {
	m, err := MatcherFromJSON([]byte(jsonConfig))
	require.NoError(t, err)
	assert.True(t, m.Matches(knownCorpus))
	assert.True(t, m.Matches(knownCorpusModelSDK))
	assert.True(t, m.Matches(knownCorpusModelSDK2))
	assert.True(t, m.Matches(knownCorpus_AmbivalentSDK))

	assert.False(t, m.Matches(knownCorpusModel_UnknownSDK))
	assert.False(t, m.Matches(knownCorpusSDK_UnknownModel))
	assert.False(t, m.Matches(knownCorpus_UnknownModelSDK))
	assert.False(t, m.Matches(knownModelSDK_UnknownCorpus))
	assert.False(t, m.Matches(knownCorpusModel_MissingSDK))
	assert.False(t, m.Matches(knownCorpus_MissingModelSDK))
	assert.False(t, m.Matches(knownModelSDK_MissingCorpus))
	assert.False(t, m.Matches(knownCorpusModel_EmptySDK))
	assert.False(t, m.Matches(nil))
}

func TestMatcherFromJSON_EmptyConfig_ReturnsError(t *testing.T) {
	_, err := MatcherFromJSON([]byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No rules detected")
}

func TestMatcherFromJSON_InvalidJSON_ReturnsError(t *testing.T) {
	_, err := MatcherFromJSON([]byte("This is not JSON"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}
