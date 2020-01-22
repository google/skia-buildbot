package ignore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
)

func TestAsMatcherSunnyDay(t *testing.T) {
	unittest.SmallTest(t)
	r1 := NewRule("jon@example.com", time.Now().Add(time.Hour), "config=gpu", "good")
	r2 := NewRule("jon@example.com", time.Now().Add(time.Hour), "os=linux&config=cpu", "reason")

	m, err := AsMatcher([]Rule{r1, r2})
	require.NoError(t, err)
	// matches the first rule
	assert.True(t, m.MatchAnyParams(paramtools.Params{
		"config": "gpu",
	}))
	// matches the second rule
	assert.True(t, m.MatchAnyParams(paramtools.Params{
		"config": "cpu",
		"os":     "linux",
	}))
	// matches the first rule, but with some extra
	assert.True(t, m.MatchAnyParams(paramtools.Params{
		"config":  "gpu",
		"snicker": "poodle",
	}))
	// completely wrong
	assert.False(t, m.MatchAnyParams(paramtools.Params{
		"snicker": "doodle",
	}))
	// almost matches second rule, but not an exact match
	assert.False(t, m.MatchAnyParams(paramtools.Params{
		"os": "linux",
	}))
}

func TestAsMatcherEmpty(t *testing.T) {
	unittest.SmallTest(t)
	queries, err := AsMatcher([]Rule{})
	require.NoError(t, err)
	assert.Len(t, queries, 0)
}

func TestAsMatcherInvalidRule(t *testing.T) {
	unittest.SmallTest(t)
	// A bad rule won't get converted
	r := NewRule("jon@example.com", time.Now().Add(time.Hour), "bad=%", "reason")
	_, err := AsMatcher([]Rule{r})
	require.Error(t, err)
}

func TestFilterIgnored(t *testing.T) {
	unittest.SmallTest(t)

	// With no ignore rules, nothing is filtered
	ft, pm, err := FilterIgnored(data.MakeTestTile(), nil)
	require.NoError(t, err)
	require.Empty(t, pm)
	require.Equal(t, data.MakeTestTile(), ft)

	future := time.Now().Add(time.Hour)
	ignores := []Rule{
		NewRule("user@example.com", future, "device=crosshatch", "note"),
	}

	// Now filter the tile and make sure those traces are filtered out.
	ft, pm, err = FilterIgnored(data.MakeTestTile(), ignores)
	require.NoError(t, err)
	require.Equal(t, paramtools.ParamMatcher{
		{
			"device": {"crosshatch"},
		},
	}, pm)
	require.Len(t, ft.Traces, 4)
	require.NotContains(t, ft.Traces, data.CrosshatchAlphaTraceID)
	require.NotContains(t, ft.Traces, data.CrosshatchBetaTraceID)
}
