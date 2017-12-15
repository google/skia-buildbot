package flaky

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/suggester/go/dsconst"
)

func TestTimeRange(t *testing.T) {
	testutils.SmallTest(t)

	now := time.Now()
	tr := TimeRange{
		Begin: now.Add(-1 * time.Hour),
		End:   now,
	}
	assert.False(t, tr.In(now.Add(-2*time.Hour)))
	assert.False(t, tr.In(now.Add(-1*time.Hour)))
	assert.True(t, tr.In(now.Add(-1*time.Minute)))
	assert.False(t, tr.In(now))
}

func TestFlaky(t *testing.T) {
	now := time.Now()
	f := Flaky{
		"Bot-1": []*TimeRange{
			&TimeRange{now.Add(-5 * time.Hour), now.Add(-4 * time.Hour)},
			&TimeRange{now.Add(-1 * time.Hour), now.Add(-30 * time.Minute)},
		},
		"Bot-2": []*TimeRange{
			&TimeRange{now.Add(-4 * time.Hour), now.Add(-3 * time.Hour)},
		},
	}
	assert.False(t, f.WasFlaky("unknown bot", now))
	assert.False(t, f.WasFlaky("Bot-1", now.Add(-6*time.Hour)))
	assert.True(t, f.WasFlaky("Bot-1", now.Add(-45*time.Minute)))
	assert.True(t, f.WasFlaky("Bot-1", now.Add(-241*time.Minute)))
	assert.False(t, f.WasFlaky("Bot-1", now.Add(-1*time.Minute)))

	assert.False(t, f.WasFlaky("Bot-2", now.Add(-1*time.Minute)))
	assert.True(t, f.WasFlaky("Bot-2", now.Add(-181*time.Minute)))

}

const BOTNAME = "Test-Chromecast-GCC-Chorizo-CPU-Cortex_A7-arm-Release-All"

func TestFlakyReadWrite(t *testing.T) {
	testutils.MediumTest(t)

	cleanup := testutil.InitDatastore(t, dsconst.FLAKY_RANGES)
	defer cleanup()

	provider := func() (map[string]time.Time, error) { return nil, nil }

	fb := NewFlakyBuilder(provider, ds.DS)

	// Add a range that ends 3 hours ago.
	now := time.Now().UTC()
	begin := now.Add(-5 * time.Hour)
	err := fb.createOrUpdateFlaky(BOTNAME, begin, now.Add(-3*time.Hour), true)
	assert.NoError(t, err)

	// Test that querying works.
	flaky, err := fb.Build(24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)

	flaky, err = fb.Build(1*time.Minute, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 0)

	// Update the existing range's endpoint.
	err = fb.createOrUpdateFlaky(BOTNAME, begin, now.Add(-2*time.Hour), true)
	assert.NoError(t, err)

	// Query again.
	flaky, err = fb.Build(4*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)
	assert.Equal(t, flaky[BOTNAME][0].Begin.Unix(), begin.Unix())

	// Close the range.
	err = fb.createOrUpdateFlaky(BOTNAME, begin, now.Add(-2*time.Hour), false)
	assert.NoError(t, err)

	// Query again.
	flaky, err = fb.Build(4*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)

	// Add a new range that ends 2 minutes ago.
	err = fb.createOrUpdateFlaky(BOTNAME, now.Add(-1*time.Hour), now.Add(-2*time.Minute), true)
	assert.NoError(t, err)

	// Query that should get both.
	flaky, err = fb.Build(24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky[BOTNAME], 2)
}
