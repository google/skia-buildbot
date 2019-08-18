package repograph

/*
   This file contains non-Graph helper functions.
*/

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
)

// RecurseLongCommits behaves exactly like Recurse, except it traverses the
// given map of LongCommits, starting with the given hash. Unlike Recurse,
// it may return an error in the case of missing commits.
func RecurseLongCommits(commits map[string]*vcsinfo.LongCommit, start string, fn func(*vcsinfo.LongCommit) error) error {
	c, ok := commits[start]
	if !ok {
		return skerr.Fmt("RecurseLongCommits: commit %s is missing.", start)
	}
	return recurseLongCommits(c, commits, fn, make(map[*vcsinfo.LongCommit]bool, len(commits)))
}

// recurseLongCommits is a helper function for RecurseLongCommits.
func recurseLongCommits(c *vcsinfo.LongCommit, commits map[string]*vcsinfo.LongCommit, fn func(*vcsinfo.LongCommit) error, visited map[*vcsinfo.LongCommit]bool) error {
	// TODO(borenet): Is it possible to share this implementation with
	// recurse()?
	for {
		visited[c] = true
		if err := fn(c); err == ErrStopRecursing {
			return nil
		} else if err != nil {
			return err
		}
		if len(c.Parents) == 0 {
			return nil
		} else if len(c.Parents) == 1 {
			p, ok := commits[c.Parents[0]]
			if !ok {
				return skerr.Fmt("RecurseLongCommits: commit %s is missing.", c.Parents[0])
			}
			if visited[p] {
				return nil
			}
			c = p
		} else {
			break
		}
	}
	if len(c.Parents) > 1 {
		for _, hash := range c.Parents {
			p, ok := commits[hash]
			if !ok {
				return skerr.Fmt("RecurseLongCommits: commit %s is missing.", c.Parents[0])
			}
			if visited[p] {
				continue
			}
			if err := recurseLongCommits(p, commits, fn, visited); err != nil {
				return err
			}
		}
	}
	return nil
}
