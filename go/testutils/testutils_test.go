package testutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sktest"
)

func TestInterfaces(t *testing.T) {

	// Ensure that our interfaces are compatible.
	var _ assert.TestingT = sktest.TestingT(nil)
	var _ sktest.TestingT = (*testing.T)(nil)
	var _ sktest.TestingT = (*testing.B)(nil)
}

func TestReadFileBytes_FileExists_Success(t *testing.T) {
	require.Equal(t, "my test data", string(ReadFileBytes(t, "mytestdata.txt")))
}
