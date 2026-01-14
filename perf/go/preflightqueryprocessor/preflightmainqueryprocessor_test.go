package preflightqueryprocessor

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
)

func TestProcessTraceIds_MissingKeysAreAddedAsEmptyStrings(t *testing.T) {
	q, err := query.New(url.Values{"benchmark": []string{"b1"}})
	assert.NoError(t, err)

	processor := NewPreflightMainQueryProcessor(q)
	processor.SetKeysToDetectMissing([]string{"bot", "test"})

	inputParams := []paramtools.Params{
		// Trace 1: Has 'bot', missing 'test'
		{"benchmark": "b1", "bot": "bot1"},
		// Trace 2: Has 'test', missing 'bot'
		{"benchmark": "b1", "test": "test1"},
		// Trace 3: Has both
		{"benchmark": "b1", "bot": "bot2", "test": "test2"},
		// Trace 4: Missing both
		{"benchmark": "b1"},
	}

	inputChannel := make(chan paramtools.Params, len(inputParams))
	for _, p := range inputParams {
		inputChannel <- p
	}
	close(inputChannel)

	processedParams := processor.ProcessTraceIds(inputChannel)

	assert.Len(t, processedParams, 4)

	// Verify Trace 1
	assert.Equal(t, "bot1", processedParams[0]["bot"])
	_, ok := processedParams[0]["test"]
	assert.False(t, ok, "key 'test' should be missing in Trace 1 params")

	// Verify Trace 2
	_, ok = processedParams[1]["bot"]
	assert.False(t, ok, "key 'bot' should be missing in Trace 2 params")
	assert.Equal(t, "test1", processedParams[1]["test"])

	// Verify Trace 3
	assert.Equal(t, "bot2", processedParams[2]["bot"])
	assert.Equal(t, "test2", processedParams[2]["test"])

	// Verify Trace 4
	_, ok = processedParams[3]["bot"]
	assert.False(t, ok, "key 'bot' should be missing in Trace 4 params")
	_, ok = processedParams[3]["test"]
	assert.False(t, ok, "key 'test' should be missing in Trace 4 params")

	// Verify ParamSet aggregation
	paramSet := processor.GetParamSet()
	assert.Contains(t, (*paramSet)["bot"], "")
	assert.Contains(t, (*paramSet)["test"], "")

	// Verify count
	assert.Equal(t, 4, processor.GetCount())
}

func TestMainQuery_SentinelMixed(t *testing.T) {
	// Query: a=[v1, __missing__]
	values := url.Values{
		"a": []string{"v1", MissingValueSentinel},
		"b": []string{"v2"},
	}
	q, err := query.New(values)
	assert.NoError(t, err)

	// Verify initial query has sentinel
	assert.Contains(t, q.Params[0].Values, MissingValueSentinel)

	// Create processor (this should clone and modify query)
	p := NewPreflightMainQueryProcessor(q)

	// Verify original query is UNTOUCHED
	assert.Contains(t, q.Params[0].Values, MissingValueSentinel, "Original query should not be modified")

	// Verify internal query has key REMOVED (superset fetch)
	internalQ := p.GetQuery()
	for _, param := range internalQ.Params {
		if param.Key() == "a" {
			t.Errorf("Key 'a' should have been removed from internal query")
		}
	}

	inputChannel := make(chan paramtools.Params, 3)
	// Trace 1: a=v1. Should match.
	inputChannel <- paramtools.Params{"a": "v1", "b": "v2"}
	// Trace 2: a=missing. Should match.
	inputChannel <- paramtools.Params{"b": "v2"}
	// Trace 3: a=v3. Should NOT match.
	inputChannel <- paramtools.Params{"a": "v3", "b": "v2"}
	close(inputChannel)

	out := p.ProcessTraceIds(inputChannel)

	assert.Len(t, out, 2)
	assert.Equal(t, "v1", out[0]["a"])
	_, ok := out[1]["a"]
	assert.False(t, ok)
}

func TestMainQuery_SentinelOnly(t *testing.T) {
	// Query: a=__missing__
	values := url.Values{
		"a": []string{MissingValueSentinel},
	}
	q, err := query.New(values)
	assert.NoError(t, err)

	p := NewPreflightMainQueryProcessor(q)

	inputChannel := make(chan paramtools.Params, 2)
	inputChannel <- paramtools.Params{"b": "v2"}            // Missing "a". Match.
	inputChannel <- paramtools.Params{"a": "v1", "b": "v2"} // Has "a". No Match.
	close(inputChannel)

	out := p.ProcessTraceIds(inputChannel)
	assert.Len(t, out, 1)
	_, ok := out[0]["a"]
	assert.False(t, ok)
}

func TestSubQuery_Sentinel(t *testing.T) {
	// SubQuery logic: Collect values for key "b", given filter on "a".
	// Query: a=[v1, __missing__]
	values := url.Values{
		"a": []string{"v1", MissingValueSentinel},
	}
	q, err := query.New(values)
	assert.NoError(t, err)

	// Create a dummy main processor (required for shared state)
	mainP := NewPreflightMainQueryProcessor(q)

	// Create sub processor for key "b"
	// SubQuery uses same query params as main query
	subP := NewPreflightSubQueryProcessor(mainP, q, "b")

	inputChannel := make(chan paramtools.Params, 4)
	// Trace 1: a=v1, b=val1. Match.
	inputChannel <- paramtools.Params{"a": "v1", "b": "val1"}
	// Trace 2: a=missing, b=val2. Match.
	inputChannel <- paramtools.Params{"b": "val2"}
	// Trace 3: a=v2, b=val3. No Match.
	inputChannel <- paramtools.Params{"a": "v2", "b": "val3"}
	// Trace 4: a=v1, b missing. Match (but no value for b).
	inputChannel <- paramtools.Params{"a": "v1"}
	close(inputChannel)

	// Process
	subP.ProcessTraceIds(inputChannel)
	subP.Finalize()

	// Verify ParamSet in main processor (shared)
	ps := mainP.GetParamSet()
	assert.Contains(t, (*ps)["b"], "val1")
	assert.Contains(t, (*ps)["b"], "val2")
	assert.NotContains(t, (*ps)["b"], "val3")
}
