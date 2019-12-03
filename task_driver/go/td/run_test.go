package td

import (
	"testing"

	deepequal_testutils "go.skia.org/infra/go/deepequal/testutils"
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
	deepequal_testutils.AssertCopy(t, p, p.Copy())
}
