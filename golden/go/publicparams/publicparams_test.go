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
	assert.True(t, ev.Matches(nil))
}

func TestMatcherFromJSON_FollowsConfig(t *testing.T) {
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
	assert.False(t, m.Matches(nil))
}
