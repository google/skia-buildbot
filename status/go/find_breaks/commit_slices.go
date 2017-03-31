package find_breaks

import (
	"time"

	"go.skia.org/infra/go/git/repograph"
)

// commitSlicesForCommit finds slices of commits, creating duplicate histories
// for branches and merges, so that users can assume single-branch history. It
// starts at the given commit and runs recursively.
func commitSlicesForCommit(c *repograph.Commit, start, end time.Time) [][]string {
	slice := []string{}
	for {
		if start.After(c.Timestamp) {
			break
		}
		if c.Timestamp.Before(end) {
			slice = append(slice, c.Hash)
		}
		p := c.GetParents()
		if len(p) == 0 {
			break
		}
		if len(p) == 1 {
			c = p[0]
		} else {
			rv := [][]string{}
			for _, parent := range p {
				slices := commitSlicesForCommit(parent, start, end)
				for _, s := range slices {
					rv = append(rv, append(slice, s...))
				}
			}
			return rv
		}
	}
	if len(slice) == 0 {
		return [][]string{}
	}
	return [][]string{slice}
}

// commitSlicesForCommit finds slices of commits, creating duplicate histories
// for branches and merges, so that users can assume single-branch history.
//
// For example, this repo:
//
//      d   c
//      | /
//      b
//      |
//      a
//
// Would give us these slices:
//
//      d   c
//      |   |
//      b   b
//      |   |
//      a   a
//
func commitSlices(repo *repograph.Graph, start, end time.Time) [][]string {
	rv := [][]string{}
	for _, b := range repo.Branches() {
		slices := commitSlicesForCommit(repo.Get(b), start, end)
		rv = append(rv, slices...)
	}
	// Reverse all slices so that they're in chronological order.
	for _, slice := range rv {
		for i := 0; i < len(slice)/2; i++ {
			j := len(slice) - i - 1
			slice[i], slice[j] = slice[j], slice[i]
		}
	}
	return rv
}
