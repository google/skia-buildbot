package windows

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestOSVersions_HappyPath(t *testing.T) {
	unittest.SmallTest(t)
	versions, err := OSVersions("Microsoft Windows Server 2019 Datacenter", "10.0.17763 Build 17763")
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"Windows", "Windows-Server", "Windows-Server-17763"},
		versions,
	)
}

func TestOSVersions_CantParsePlatform_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := OSVersions("Schlockosoft Grindows", "10.0.17763 Build 17763")
	require.Error(t, err)
}
