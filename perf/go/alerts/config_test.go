package alerts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestConfig(t *testing.T) {
	unittest.SmallTest(t)

	a := NewConfig()
	assert.Equal(t, "-1", a.IDAsString)
	a.SetIDFromString("2")
	assert.Equal(t, "2", a.IDAsString)
}

func TestStringToID(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, int64(-1), IDAsStringToInt("foo"))
	assert.Equal(t, int64(12), IDAsStringToInt("12"))
	assert.Equal(t, int64(-1), IDAsStringToInt("-1"))
	assert.Equal(t, int64(-1), IDAsStringToInt(""))
}

func TestValidate(t *testing.T) {
	unittest.SmallTest(t)
	a := NewConfig()
	assert.NoError(t, a.Validate())

	assert.Equal(t, BOTH, a.DirectionAsString)
	a.StepUpOnly = true
	assert.NoError(t, a.Validate())
	assert.False(t, a.StepUpOnly)
	assert.Equal(t, UP, a.DirectionAsString)

	a.GroupBy = "foo"
	assert.NoError(t, a.Validate())
	a.Query = "bar=baz"
	assert.NoError(t, a.Validate())

	a.GroupBy = "foo,quux"
	a.Query = "bar=baz"
	assert.NoError(t, a.Validate())

	a.GroupBy = "bar,quux"
	a.Query = "quux=baz"
	assert.Error(t, a.Validate())

	a.GroupBy = "foo"
	a.Query = "bar=baz&foo=quux"
	assert.Error(t, a.Validate())
}

func TestGroupedBy(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		value    string
		expected []string
		message  string
	}{
		{
			value:    "model",
			expected: []string{"model"},
			message:  "Simple",
		},
		{
			value:    "model,branch",
			expected: []string{"model", "branch"},
			message:  "Two",
		},
		{
			value:    ",model , branch, \n",
			expected: []string{"model", "branch"},
			message:  "Two with extra junk.",
		},
		{
			value:    " \n",
			expected: []string{},
			message:  "Just whitespace",
		},
		{
			value:    "",
			expected: []string{},
			message:  "empty",
		},
	}

	for _, tc := range testCases {
		cfg := &Alert{GroupBy: tc.value}
		assert.Equal(t, tc.expected, cfg.GroupedBy(), tc.message)
	}
}

func TestGroupCombinations(t *testing.T) {
	unittest.SmallTest(t)
	ps := paramtools.ParamSet{
		"model":  []string{"nexus4", "nexus6", "nexus6"},
		"config": []string{"565", "8888", "nvpr"},
		"arch":   []string{"ARM", "x86"},
	}
	ps.Normalize()
	cfg := &Alert{
		GroupBy: "foo, config",
	}
	_, err := cfg.GroupCombinations(paramtools.ReadOnlyParamSet(ps))
	assert.Error(t, err, "Unknown key")

	cfg = &Alert{
		GroupBy: "arch, config",
	}
	actual, err := cfg.GroupCombinations(paramtools.ReadOnlyParamSet(ps))
	assert.NoError(t, err)
	expected := []Combination{
		{KeyValue{Key: "arch", Value: "x86"}, KeyValue{Key: "config", Value: "nvpr"}},
		{KeyValue{Key: "arch", Value: "ARM"}, KeyValue{Key: "config", Value: "nvpr"}},
		{KeyValue{Key: "arch", Value: "x86"}, KeyValue{Key: "config", Value: "8888"}},
		{KeyValue{Key: "arch", Value: "ARM"}, KeyValue{Key: "config", Value: "8888"}},
		{KeyValue{Key: "arch", Value: "x86"}, KeyValue{Key: "config", Value: "565"}},
		{KeyValue{Key: "arch", Value: "ARM"}, KeyValue{Key: "config", Value: "565"}},
	}
	assert.Equal(t, expected, actual)
}

func TestQueriesFromParamset(t *testing.T) {
	unittest.SmallTest(t)
	ps := paramtools.ParamSet{
		"model":  []string{"nexus4", "nexus6", "nexus6"},
		"config": []string{"565", "8888", "nvpr"},
		"arch":   []string{"ARM", "x86"},
	}
	ps.Normalize()
	cfg := &Alert{
		GroupBy: "foo, config",
	}
	_, err := cfg.GroupCombinations(paramtools.ReadOnlyParamSet(ps))
	assert.Error(t, err, "Unknown key")

	cfg = &Alert{
		GroupBy: "arch, config",
		Query:   "model=nexus6",
	}
	queries, err := cfg.QueriesFromParamset(paramtools.ReadOnlyParamSet(ps))
	assert.NoError(t, err)
	expected := []string{
		"arch=x86&config=nvpr&model=nexus6",
		"arch=ARM&config=nvpr&model=nexus6",
		"arch=x86&config=8888&model=nexus6",
		"arch=ARM&config=8888&model=nexus6",
		"arch=x86&config=565&model=nexus6",
		"arch=ARM&config=565&model=nexus6",
	}
	assert.Equal(t, expected, queries)

	// No GroupBy
	cfg = &Alert{
		Query: "model=nexus6",
	}
	queries, err = cfg.QueriesFromParamset(paramtools.ReadOnlyParamSet(ps))
	assert.NoError(t, err)
	assert.Equal(t, []string{"model=nexus6"}, queries)

}

func TestConfigStateToInt_Success(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, 0, ConfigStateToInt(ACTIVE))
	assert.Equal(t, 1, ConfigStateToInt(DELETED))
	assert.Equal(t, 0, ConfigStateToInt("INVALID STATE"), "Invalid ConfigState value.")
}

func TestSetIDFromInt64_GoodAlertID_Success(t *testing.T) {
	unittest.SmallTest(t)
	cfg := NewConfig()
	cfg.SetIDFromInt64(12)
	require.Equal(t, "12", cfg.IDAsString)
}

func TestSetIDFromInt64_BadAlertID_Success(t *testing.T) {
	unittest.SmallTest(t)
	cfg := NewConfig()
	cfg.SetIDFromInt64(BadAlertID)
	require.Equal(t, BadAlertIDAsAsString, cfg.IDAsString)
}

func TestIDAsStringToInt_ValidID_Success(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, int64(12), IDAsStringToInt("12"))
}

func TestIDAsStringToInt_InvalidID_Success(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, BadAlertID, IDAsStringToInt("not-a-number"))
}
