package alerts

import (
	"testing"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	testutils.SmallTest(t)

	a := NewConfig()
	assert.Equal(t, "-1", a.IdAsString())
	a.StringToId("2")
	assert.Equal(t, int64(2), a.ID)
	assert.Equal(t, "2", a.IdAsString())
}

func TestValidate(t *testing.T) {
	testutils.SmallTest(t)
	a := NewConfig()
	assert.NoError(t, a.Validate())

	assert.Equal(t, BOTH, a.Direction)
	a.StepUpOnly = true
	assert.NoError(t, a.Validate())
	assert.False(t, a.StepUpOnly)
	assert.Equal(t, UP, a.Direction)

	a.GroupBy = "foo"
	assert.NoError(t, a.Validate())
	a.Query = "bar=baz"
	assert.NoError(t, a.Validate())
	a.Query = "bar=baz&foo=quux"
	assert.Error(t, a.Validate())
}

func TestGroupedBy(t *testing.T) {
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
		cfg := &Config{GroupBy: tc.value}
		assert.Equal(t, tc.expected, cfg.GroupedBy(), tc.message)
	}
}
