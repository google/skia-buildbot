package standalone

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Smoke-test CPUs(). The interesting (and hopefully thus the error-prone) parts of it have been
// factored out so they can be tested on any CI platform, but this covers the platform-specific
// straight line through, determined by the platform the tests are running on.
func TestCPUs_Smoke(t *testing.T) {
	cpus, err := CPUs(context.Background())
	assert.NoError(t, err)
	if len(cpus) != 2 && len(cpus) != 3 {
		assert.Fail(t, "Length of CPUs() output should have at least an ISA and a bit-width element.")
	}
	if !strings.Contains(cpus[1], "-64") {
		assert.Fail(t, "CPUs()' bit-width return value should probably say 64 bits. It's unlikely the machine running this test is anything but 64 bits.")
	}
}

func TestOSVersions_Smoke(t *testing.T) {
	versions, err := OSVersions(context.Background())
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(versions), 2, "OSVersions() should return at least PlatformName and PlatformName-SomeVersion.")
}
