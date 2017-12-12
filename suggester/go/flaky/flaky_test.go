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

}
