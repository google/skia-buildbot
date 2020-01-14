package ignore

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
)

func TestToQuery(t *testing.T) {
	unittest.SmallTest(t)
	queries, err := toQuery([]Rule{})
	require.NoError(t, err)
	require.Len(t, queries, 0)

	r1 := NewRule("jon@example.com", time.Now().Add(time.Hour), "config=gpu", "reason")
	queries, err = toQuery([]Rule{r1})
	require.NoError(t, err)
	require.Equal(t, queries[0], url.Values{"config": []string{"gpu"}})

	// A bad rule won't get converted
	r1 = NewRule("jon@example.com", time.Now().Add(time.Hour), "bad=%", "reason")
	queries, err = toQuery([]Rule{r1})
	require.NotNil(t, err)
	require.Empty(t, queries)
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
