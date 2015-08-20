package search

import (
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/paramsets"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tally"
)

func SelfTest(storages *storage.Storage, tallies *tally.Tallies, blamer *blame.Blamer, paramset *paramsets.Summary) {
	// TODO (stephana): Add more tests for the Search function.
}
