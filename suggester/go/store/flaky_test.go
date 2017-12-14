package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/suggester/go/dsconst"
)

const BOTNAME = "Test-Chromecast-GCC-Chorizo-CPU-Cortex_A7-arm-Release-All"

func TestFlakyReadWrite(t *testing.T) {
	testutils.MediumTest(t)

	cleanup := testutil.InitDatastore(t, dsconst.FLAKY_RANGES)
	defer cleanup()

	// Add a range that ends 3 hours ago.
	now := time.Now().UTC()
	begin := now.Add(-5 * time.Hour)
	err := CreateOrUpdateFlaky(BOTNAME, begin, now.Add(-3*time.Hour), true)
	assert.NoError(t, err)

	// Test that querying works.
	flaky, err := ReadFlaky(24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)

	flaky, err = ReadFlaky(1*time.Minute, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 0)

	// Update the existing range's endpoint.
	err = CreateOrUpdateFlaky(BOTNAME, begin, now.Add(-2*time.Hour), true)
	assert.NoError(t, err)

	// Query again.
	flaky, err = ReadFlaky(4*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)
	assert.Equal(t, flaky[BOTNAME][0].Begin.Unix(), begin.Unix())

	// Close the range.
	err = CreateOrUpdateFlaky(BOTNAME, begin, now.Add(-2*time.Hour), false)
	assert.NoError(t, err)

	// Query again.
	flaky, err = ReadFlaky(4*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)

	// Add a new range that ends 2 minutes ago.
	err = CreateOrUpdateFlaky(BOTNAME, now.Add(-1*time.Hour), now.Add(-2*time.Minute), true)
	assert.NoError(t, err)

	// Query that should get both.
	flaky, err = ReadFlaky(24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky[BOTNAME], 2)
}
