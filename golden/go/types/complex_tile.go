package types

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/tiling"
)

// ComplexTile contains an enriched version of a tile loaded through the ingestion process.
// It provides ways to handle sparse tiles, where many commits of the underlying raw tile
// contain no data and therefore removed.
// In either case (sparse or dense tile) it offers two versions of the tile.
// one with all ignored traces and one without the ignored traces.
// In addition it also contains the ignore rules and information about the larger "sparse" tile
// if the tiles at hand were condensed from a larger tile.
type ComplexTile interface {
	// AllCommits returns all commits that were processed to get the data commits.
	// Its first commit should match the first commit returned when calling DataCommits.
	AllCommits() []*tiling.Commit

	// DataCommits returns all commits that contain data. In some busy repos, there are commits that
	// don't get tested directly because the commits are batched in with others. DataCommits
	// is a way to get just the commits where some data has been ingested.
	DataCommits() []*tiling.Commit

	// FilledCommits returns how many commits in the tile have data.
	FilledCommits() int

	// GetTile returns a simple tile either with or without ignored traces depending on the argument.
	// TODO(kjlubick) Maybe diverge from the map of traces and instead of a slice, so we can
	//  query things in parallel more easily.
	GetTile(is IgnoreState) *tiling.Tile

	// SetIgnoreRules adds ignore rules to the tile and a sub-tile with the ignores removed.
	// In other words this function assumes that original tile has been filtered by the
	// ignore rules that are being passed.
	SetIgnoreRules(reducedTile *tiling.Tile, ignoreRules paramtools.ParamMatcher)

	// IgnoreRules returns the ignore rules for this tile.
	IgnoreRules() paramtools.ParamMatcher

	// SetSparse tells the tile what the full range of commits analyzed was.
	SetSparse(allCommits []*tiling.Commit)
}

type ComplexTileImpl struct {
	// tileExcludeIgnoredTraces is the current tile without ignored traces.
	tileExcludeIgnoredTraces *tiling.Tile

	// tileIncludeIgnoredTraces is the current tile containing all available data.
	tileIncludeIgnoredTraces *tiling.Tile

	// ignoreRules contains the rules used to created the TileWithIgnores.
	ignoreRules paramtools.ParamMatcher

	// sparseCommits are all the commits that were used condense the underlying tile.
	sparseCommits []*tiling.Commit
}

func NewComplexTile(completeTile *tiling.Tile) *ComplexTileImpl {
	return &ComplexTileImpl{
		tileExcludeIgnoredTraces: completeTile,
		tileIncludeIgnoredTraces: completeTile,
	}
}

// SetIgnoreRules fulfills the ComplexTile interface.
func (c *ComplexTileImpl) SetIgnoreRules(reducedTile *tiling.Tile, ignoreRules paramtools.ParamMatcher) {
	c.tileExcludeIgnoredTraces = reducedTile
	c.ignoreRules = ignoreRules
}

// SetSparse fulfills the ComplexTile interface.
func (c *ComplexTileImpl) SetSparse(sparseCommits []*tiling.Commit) {
	c.sparseCommits = sparseCommits
}

// FilledCommits fulfills the ComplexTile interface.
func (c *ComplexTileImpl) FilledCommits() int {
	return len(c.DataCommits())
}

// DataCommits fulfills the ComplexTile interface.
func (c *ComplexTileImpl) DataCommits() []*tiling.Commit {
	return c.tileIncludeIgnoredTraces.Commits
}

// AllCommits fulfills the ComplexTile interface.
func (c *ComplexTileImpl) AllCommits() []*tiling.Commit {
	return c.sparseCommits
}

// GetTile fulfills the ComplexTile interface.
func (c *ComplexTileImpl) GetTile(is IgnoreState) *tiling.Tile {
	if is == IncludeIgnoredTraces {
		return c.tileIncludeIgnoredTraces
	}
	return c.tileExcludeIgnoredTraces
}

// IgnoreRules fulfills the ComplexTile interface.
func (c *ComplexTileImpl) IgnoreRules() paramtools.ParamMatcher {
	return c.ignoreRules
}

// Make sure ComplexTileImpl fulfills the ComplexTile Interface
var _ ComplexTile = (*ComplexTileImpl)(nil)
