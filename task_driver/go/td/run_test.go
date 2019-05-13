package td

import (
	"testing"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestCopyRunProperties(t *testing.T) {
	unittest.SmallTest(t)
	p := &RunProperties{
		Local:          true,
		SwarmingBot:    "bot",
		SwarmingServer: "server",
		SwarmingTask:   "task",
	}
	deepequal.AssertCopy(t, p, p.Copy())
}
