// Package validator provides tools for validating Tiles and the Traces they contain.
package validator

import (
	"fmt"
	"time"
)

import "skia.googlesource.com/buildbot.git/perf/go/types"

// validateTile validates a tile by confirming that all the commits are in
// ascending order.
//
// Also checks that none of the commits overlap with the following tile by
// making sure each commit appears before oldestTS.
func validateTile(tile *types.Tile, oldestTS int64, verbose, echoHashes bool) error {
	var lastTS int64 = 0
	lastHash := ""
	if verbose {
		fmt.Println("Number of Commits:", len(tile.Commits))
	}
	for i, c := range tile.Commits {
		if echoHashes {
			fmt.Println("Hash:", c.Hash, c.CommitTime)
		}
		if c.CommitTime == 0 {
			continue
		}
		if c.CommitTime > oldestTS {
			fmt.Printf("ERROR: Tiles out of order: Commit (%s) %d timestamp is %s, which appears after %s\n", c.Hash, i, time.Unix(c.CommitTime, 0), time.Unix(oldestTS, 0))
		}
		if c.CommitTime < lastTS {
			return fmt.Errorf("Commits out of order: Commit (%s) %d timestamp is %s, which appears before (%s) %s\n", c.Hash, i, time.Unix(c.CommitTime, 0), lastHash, time.Unix(lastTS, 0))
		}
		lastTS = c.CommitTime
		lastHash = c.Hash
	}

	if verbose {
		fmt.Println("Number of traces:", len(tile.Traces))
	}

	// Make sure each Trace is the right length.
	numCommits := len(tile.Commits)
	for key, trace := range tile.Traces {
		if trace.Len() != numCommits {
			return fmt.Errorf("Trace length incorrect: Num Commits %d != Values in trace %d for Key %s", numCommits, trace.Len(), key)
		}
	}

	return nil
}

// ValidateDataset validates all the tiles stored in a TileStore.
func ValidateDataset(store types.TileStore, verbose, echoHashes bool) bool {
	index := -1
	isValid := true
	// If tilebuilding were instantaneous this might cause a false negative, but it's not.
	oldestTS := time.Now().Unix()

	for {
		tile, err := store.Get(0, index)
		if err != nil {
			fmt.Printf("Failed to Get(0, %d): %s\n", index, err)
			isValid = false
			break
		}
		if verbose {
			fmt.Println("TileIndex:", tile.TileIndex)
			fmt.Println("Tile range:", tile.Commits[0].CommitTime, tile.Commits[len(tile.Commits)-1].CommitTime)
			fmt.Println("Tile range:", time.Unix(tile.Commits[0].CommitTime, 0), time.Unix(tile.Commits[len(tile.Commits)-1].CommitTime, 0))
		}
		// Validate the git hashes in the tile.
		err = validateTile(tile, oldestTS, verbose, echoHashes)
		oldestTS = tile.Commits[0].CommitTime
		if err != nil {
			fmt.Printf("Failed to validate tile %d scale 0: %s\n", index, err)
			isValid = false
			break
		}
		if index > 0 && index != tile.TileIndex {
			fmt.Printf("Tile index inconsistent: index %d != tile.TileIndex %d\n", index, tile.TileIndex)
			isValid = false
			break
		}
		if tile.Scale != 0 {
			fmt.Printf("Tile scale isn't 0: tile.Scale %d\n", tile.Scale)
			isValid = false
			break
		}
		if tile.TileIndex > 0 {
			index = tile.TileIndex - 1
		} else {
			break
		}
	}

	return isValid
}
