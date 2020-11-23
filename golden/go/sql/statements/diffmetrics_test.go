package statements

import (
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"testing"
)

func TestCreateDiffMetricsClosestViewShard_RangeCorrectlyFormatted(t *testing.T) {
	unittest.SmallTest(t)
	statement := CreateDiffMetricsClosestViewShard(0)
	assert.Contains(t, statement, `WHERE left_digest > x'00' and left_digest < x'01'`)
	statement = CreateDiffMetricsClosestViewShard(17)
	assert.Contains(t, statement, `WHERE left_digest > x'11' and left_digest < x'12'`)
	statement = CreateDiffMetricsClosestViewShard(255)
	assert.Contains(t, statement, `WHERE left_digest > x'ff'`)
}
