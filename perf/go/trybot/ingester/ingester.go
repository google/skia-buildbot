package ingester

import (
	"go.skia.org/infra/perf/go/file"
	"go.skia.org/infra/perf/go/trybot"
)

// Ingester converts file.Files into trybot.TryFiles as they arrive.
type Ingester interface {
	// Start a background Go routine that processes the incoming channel.
	Start(<-chan file.File) (<-chan trybot.TryFile, error)
}
