package td

import (
	"testing"

	"go.skia.org/infra/go/deepequal/assertdeep"
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
	assertdeep.Copy(t, p, p.Copy())
}
