package td

import (
	"testing"

	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestCopyRunProperties(t *testing.T) {
	p := &RunProperties{
		Local:          true,
		SwarmingBot:    "bot",
		SwarmingServer: "server",
		SwarmingTask:   "task",
	}
	assertdeep.Copy(t, p, p.Copy())
}
