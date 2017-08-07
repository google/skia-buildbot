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
	assert.Equal(t, 2, a.ID)
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
