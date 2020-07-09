package alerts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestConfig(t *testing.T) {
	unittest.SmallTest(t)

	a := NewConfig()
	assert.Equal(t, "-1", a.IDToString())
	a.SetIDFromString("2")
	assert.Equal(t, int64(2), a.ID)
	assert.Equal(t, "2", a.IDToString())
}

func TestIDToString(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, "12", IDToString(12))
	assert.Equal(t, "-1", IDToString(-1))
}

func TestStringToID(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, int64(-1), StringToID("foo"))
	assert.Equal(t, int64(12), StringToID("12"))
	assert.Equal(t, int64(-1), StringToID("-1"))
	assert.Equal(t, int64(-1), StringToID(""))
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

func TestCombinations(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		value       []int
		limits      []int
		expected    []int
		equalLimits bool
		message     string
	}{
		{
			value:       []int{0, 1, 0},
			expected:    []int{0, 1, 1},
			limits:      []int{2, 2, 2},
			equalLimits: false,
			message:     "simple",
		},
		{
			value:       []int{0, 1, 1},
			expected:    []int{1, 0, 0},
			limits:      []int{1, 1, 1},
			equalLimits: false,
			message:     "Rollover",
		},
		{
			value:       []int{0, 2, 4},
			expected:    []int{0, 3, 0},
			limits:      []int{5, 3, 4},
			equalLimits: false,
			message:     "Rollover, mixed",
		},
		{
			value:       []int{0, 3, 4},
			expected:    []int{1, 0, 0},
			limits:      []int{5, 3, 4},
			equalLimits: false,
			message:     "Rollover, mixed 2",
		},
		{
			value:       []int{5, 3, 3},
			expected:    []int{5, 3, 4},
			limits:      []int{5, 3, 4},
			equalLimits: true,
			message:     "Rollover, mixed at limits",
		},
		{
			value:       []int{5, 3, 4},
			expected:    []int{0, 0, 0},
			limits:      []int{5, 3, 4},
			equalLimits: false,
			message:     "Rollover, all",
		},
		{
			value:       []int{},
			expected:    []int{},
			limits:      []int{},
			equalLimits: true,
			message:     "Empty",
		},
		{
			value:       []int{2},
			expected:    []int{3},
			limits:      []int{3},
			equalLimits: true,
			message:     "Single",
		},
		{
			value:       []int{3},
			expected:    []int{0},
			limits:      []int{3},
			equalLimits: false,
			message:     "Single rollover",
		},
	}
	for _, tc := range testCases {
		next := inc(tc.value, tc.limits)
		assert.Equal(t, tc.expected, next, tc.message)
		assert.Equal(t, tc.equalLimits, equal(tc.limits, next), tc.message)
	}
}

func TestToCombination(t *testing.T) {
	unittest.SmallTest(t)
	res, err := toCombination([]int{1, 2}, []string{"config", "model"},
		paramtools.ParamSet{
			"model":  []string{"nexus4", "nexus6", "nexus6"},
			"config": []string{"8888", "565", "nvpr"},
		})
	assert.NoError(t, err)
	expected := Combination{
		KeyValue{"config", "565"},
		KeyValue{"model", "nexus6"},
	}
	assert.Equal(t, expected, res)
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
	_, err := cfg.GroupCombinations(ps)
	assert.Error(t, err, "Unknown key")

	cfg = &Alert{
		GroupBy: "arch, config",
	}
	actual, err := cfg.GroupCombinations(ps)
	assert.NoError(t, err)
	expected := []Combination{
		{KeyValue{"arch", "ARM"}, KeyValue{"config", "565"}},
		{KeyValue{"arch", "ARM"}, KeyValue{"config", "8888"}},
		{KeyValue{"arch", "ARM"}, KeyValue{"config", "nvpr"}},
		{KeyValue{"arch", "x86"}, KeyValue{"config", "565"}},
		{KeyValue{"arch", "x86"}, KeyValue{"config", "8888"}},
		{KeyValue{"arch", "x86"}, KeyValue{"config", "nvpr"}},
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
	_, err := cfg.GroupCombinations(ps)
	assert.Error(t, err, "Unknown key")

	cfg = &Alert{
		GroupBy: "arch, config",
		Query:   "model=nexus6",
	}
	queries, err := cfg.QueriesFromParamset(ps)
	assert.NoError(t, err)
	expected := []string{
		"arch=ARM&config=565&model=nexus6",
		"arch=ARM&config=8888&model=nexus6",
		"arch=ARM&config=nvpr&model=nexus6",
		"arch=x86&config=565&model=nexus6",
		"arch=x86&config=8888&model=nexus6",
		"arch=x86&config=nvpr&model=nexus6",
	}
	assert.Equal(t, expected, queries)

	// No GroupBy
	cfg = &Alert{
		Query: "model=nexus6",
	}
	queries, err = cfg.QueriesFromParamset(ps)
	assert.NoError(t, err)
	assert.Equal(t, []string{"model=nexus6"}, queries)

}

func TestConfigStateToInt_Success(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, 0, ConfigStateToInt(ACTIVE))
	assert.Equal(t, 1, ConfigStateToInt(DELETED))
	assert.Equal(t, 0, ConfigStateToInt(ConfigState(17)), "Invalid ConfigState value.")
}
