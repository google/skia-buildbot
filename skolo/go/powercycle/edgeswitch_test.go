package powercycle

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	err := os.Setenv(powerCyclePasswordEnvVar, "bar")
	require.NoError(t, err)
	defer func() {
		err := os.Unsetenv(powerCyclePasswordEnvVar)
		require.NoError(t, err)
	}()
	assert.Equal(t, "bar", e.getPassword())
}
