package find_breaks

import (
	"time"

	"go.skia.org/infra/go/git/repograph"
)

func commitSlicesForCommit(c *repograph.Commit, start, end time.Time) [][]*repograph.Commit {
	slice := []*repograph.Commit{}
	for {
		if start.After(c.Timestamp) {
			break
		}
		if c.Timestamp.Before(end) {
			slice = append(slice, c)
		}
		p := c.GetParents()
		if len(p) == 0 {
			break
		}
		if len(p) == 1 {
			c = p[0]
		} else {
			rv := [][]*repograph.Commit{}
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
		return [][]*repograph.Commit{}
	}
	return [][]*repograph.Commit{slice}
}

func commitSlices(repo *repograph.Graph, start, end time.Time) [][]*repograph.Commit {
	rv := [][]*repograph.Commit{}
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
