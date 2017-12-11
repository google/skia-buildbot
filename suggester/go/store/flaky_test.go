package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/suggester/go/dsconst"
)

func TestFlakyReadWrite(t *testing.T) {
	testutils.MediumTest(t)

	cleanup := testutil.InitDatastore(t, dsconst.FLAKY_RANGES)
	defer cleanup()

	now := time.Now().UTC()

	err := CreateOrUpdateFlaky("Test-Chromecast-GCC-Chorizo-CPU-Cortex_A7-arm-Release-All", now.Add(-1*time.Hour), now.Add(-2*time.Minute), true)
	assert.NoError(t, err)
	flaky, err := ReadFlaky(24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)

	flaky, err = ReadFlaky(1*time.Minute, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 0)
}
