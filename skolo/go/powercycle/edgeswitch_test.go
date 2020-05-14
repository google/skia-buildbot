package powercycle

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetPassword_EmptyIfNotSet(t *testing.T) {
	unittest.SmallTest(t)

	e := EdgeSwitchConfig{}
	assert.Equal(t, "", e.getPassword())
}

func TestGetPassword_SuccessIfSetInStruct(t *testing.T) {
	unittest.SmallTest(t)

	e := EdgeSwitchConfig{
		Password: "foo",
	}
	assert.Equal(t, "foo", e.getPassword())
}

func TestGetPassword_SuccessIfSetByEnvVar(t *testing.T) {
	unittest.SmallTest(t)

	e := EdgeSwitchConfig{}
	os.Setenv(powerCyclePasswordEnvVar, "bar")
	defer os.Unsetenv(powerCyclePasswordEnvVar)
	assert.Equal(t, "bar", e.getPassword())
}
