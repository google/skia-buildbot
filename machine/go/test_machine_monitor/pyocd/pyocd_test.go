package pyocd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeviceType_UsesValueStoredInStruct(t *testing.T) {
	ctx := context.Background()
	pyocd := pyocdImpl{
		deviceType: "RT1170_EVK",
	}
	actual, err := pyocd.DeviceType(ctx)
	require.NoError(t, err)
	assert.Equal(t, "RT1170_EVK", actual)
}
