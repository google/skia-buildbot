package flaky

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
)

func TestTimeRange(t *testing.T) {
	testutils.SmallTest(t)

	now := time.Now()
	tr := TimeRange{
		Begin: now.Add(-1 * time.Hour),
		End:   now,
	}
	assert.False(t, tr.Contains(now.Add(-2*time.Hour)))
	assert.True(t, tr.Contains(now.Add(-1*time.Hour)))
	assert.True(t, tr.Contains(now.Add(-1*time.Minute)))
	assert.False(t, tr.Contains(now))
}

func TestFlaky(t *testing.T) {
	testutils.SmallTest(t)
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
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t, ds.FLAKY_RANGES)
	defer cleanup()

	provider := func() (map[string]time.Time, error) { return nil, nil }

	fb := NewFlakyBuilder(provider)

	// Add a range that ends 3 hours ago.
	ctx := context.Background()
	now := time.Now().UTC()
	begin := now.Add(-5 * time.Hour)
	err := fb.createOrUpdateFlaky(ctx, BOTNAME, begin, now.Add(-3*time.Hour))
	assert.NoError(t, err)

	// Test that querying works.
	flaky, err := fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)

	flaky, err = fb.Build(ctx, 1*time.Minute, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 0)

	// Update the existing range's endpoint.
	err = fb.createOrUpdateFlaky(ctx, BOTNAME, begin, now.Add(-2*time.Hour))
	assert.NoError(t, err)

	// Query again.
	flaky, err = fb.Build(ctx, 4*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)
	assert.Equal(t, flaky[BOTNAME][0].Begin.Unix(), begin.Unix())

	// Close the range.
	err = fb.closeFlaky(ctx, BOTNAME)
	assert.NoError(t, err)

	// Query again.
	flaky, err = fb.Build(ctx, 4*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky, 1)

	// Add a new range that ends 2 minutes ago.
	err = fb.createOrUpdateFlaky(ctx, BOTNAME, now.Add(-1*time.Hour), now.Add(-2*time.Minute))
	assert.NoError(t, err)

	// Query that should get both.
	flaky, err = fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flaky[BOTNAME], 2)
}

func TestBuilder(t *testing.T) {
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t, ds.FLAKY_RANGES)
	defer cleanup()

	now := time.Now().UTC()
	calledIndex := 0
	comments := []map[string]time.Time{
		// 1 - empty.
		map[string]time.Time{},
		// 2 - One open range.
		map[string]time.Time{
			"Bot-1": now.Add(-10 * time.Hour),
		},
		// 3 - Two open ranges.
		map[string]time.Time{
			"Bot-1": now.Add(-10 * time.Hour),
			"Bot-2": now.Add(-5 * time.Hour),
		},
		// 4 - Back to just one open range.
		map[string]time.Time{
			"Bot-2": now.Add(-5 * time.Hour),
		},
		// 5 - Open a new range for Bot-1.
		map[string]time.Time{
			"Bot-1": now.Add(-1 * time.Hour),
			"Bot-2": now.Add(-5 * time.Hour),
		},
		// 6 - Close all the ranges.
		map[string]time.Time{},
		// 7 - Open a new range for Bot-2.
		map[string]time.Time{
			"Bot-2": now.Add(-2 * time.Hour),
		},
	}
	provider := func() (map[string]time.Time, error) {
		calledIndex += 1
		return comments[calledIndex-1], nil
	}

	ctx := context.Background()
	fb := NewFlakyBuilder(provider)

	// 1 - empty.
	err := fb.Update(ctx)
	assert.NoError(t, err)
	flakes, err := fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flakes, 0)

	// 2 - One open range.
	err = fb.Update(ctx)
	assert.NoError(t, err)
	flakes, err = fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flakes, 1)
	assert.True(t, flakes.WasFlaky("Bot-1", now.Add(-8*time.Hour)))

	// 3 - Two open ranges.
	err = fb.Update(ctx)
	assert.NoError(t, err)
	flakes, err = fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flakes, 2)
	assert.True(t, flakes.WasFlaky("Bot-1", now.Add(-8*time.Hour)))
	assert.False(t, flakes.WasFlaky("Bot-1", now.Add(-11*time.Hour)))
	assert.True(t, flakes.WasFlaky("Bot-2", now.Add(-4*time.Hour)))
	assert.False(t, flakes.WasFlaky("Bot-2", now.Add(-6*time.Hour)))

	// 4 - Back to just one open range.
	err = fb.Update(ctx)
	assert.NoError(t, err)
	flakes, err = fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flakes, 2)

	// 5 - Open a new range for Bot-1.
	err = fb.Update(ctx)
	assert.NoError(t, err)
	flakes, err = fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flakes, 2)
	assert.Len(t, flakes["Bot-1"], 2)
	assert.Len(t, flakes["Bot-2"], 1)

	// 6 - Close all the ranges.
	err = fb.Update(ctx)
	assert.NoError(t, err)
	flakes, err = fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flakes, 2)
	assert.Len(t, flakes["Bot-1"], 2)
	assert.Len(t, flakes["Bot-2"], 1)

	// 7 - Open a new range for Bot-2.
	err = fb.Update(ctx)
	assert.NoError(t, err)
	flakes, err = fb.Build(ctx, 24*time.Hour, now)
	assert.NoError(t, err)
	assert.Len(t, flakes, 2)
	assert.Len(t, flakes["Bot-1"], 2)
	assert.Len(t, flakes["Bot-2"], 2)
}
