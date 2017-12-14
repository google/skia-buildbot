package flaky

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
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
