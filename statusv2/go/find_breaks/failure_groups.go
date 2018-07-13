package find_breaks

import (
	"crypto/md5"
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
)

// FailureGroup represents a group of failures which are likely to be related.
// That is, they probably share the same problem and root cause.
type FailureGroup struct {
	// Ids are the ids of all of the failures in the group.
	Ids []string

	// BrokeIn is the slice of commits which may have caused the failures in
	// the group.
	BrokeIn []string
	brokeIn slice

	// Failing is the slice of commits which are affected by the failures in
	// the group.
	Failing []string
	failing slice

	// FixedIn is the slice of commits which may have fixed the failures in
	// the group. May be empty if the failures have not yet been fixed.
	FixedIn []string
	fixedIn slice
}

// fromFailure creates a new FailureGroup instance based on the given failure
// instance.
func fromFailure(f *failure) *FailureGroup {
	return &FailureGroup{
		Ids:     []string{f.id},
		brokeIn: f.brokeIn.Copy(),
		failing: f.failing.Copy(),
		fixedIn: f.fixedIn.Copy(),
	}
}

// resolve fills in actual sets of commit hashes for the FailureGroup.
func (g *FailureGroup) resolve(s []string) {
	g.BrokeIn = g.brokeIn.Resolve(s)
	g.Failing = g.failing.Resolve(s)
	g.FixedIn = g.fixedIn.Resolve(s)
}

// Valid returns true iff the FailureGroup is valid.
func (g *FailureGroup) Valid() error {
	// All of the component slices must be valid.
	if !g.brokeIn.Valid() {
		return fmt.Errorf("FailureGroup: brokeIn is invalid: %v", g.brokeIn)
	}
	if !g.failing.Valid() {
		return fmt.Errorf("FailureGroup: failing is invalid: %v", g.failing)
	}
	if !g.fixedIn.Valid() {
		return fmt.Errorf("FailureGroup: fixedIn is invalid: %v", g.fixedIn)
	}
	// brokeIn must be a subslice of failing.
	if g.brokeIn.Overlap(g.failing).Len() != g.brokeIn.Len() {
		return fmt.Errorf("FailureGroup: brokeIn is not a subslice of failing: %v and %v", g.brokeIn, g.failing)
	}
	// brokeIn and fixedIn can't overlap.
	if g.brokeIn.Overlap(g.fixedIn).Len() != 0 {
		return fmt.Errorf("FailureGroup: brokeIn overlaps fixedIn: %v and %v", g.brokeIn, g.fixedIn)
	}
	// failing and fixedIn can't overlap.
	if g.failing.Overlap(g.fixedIn).Len() != 0 {
		return fmt.Errorf("FailureGroup: failing overlaps fixedIn: %v and %v", g.failing, g.fixedIn)
	}
	// Must have at least one ID.
	if len(g.Ids) < 1 {
		return fmt.Errorf("FailureGroup: no IDs")
	}
	return nil
}

// maybeMerge merges the failure into the FailureGroup if possible.
func (g *FailureGroup) maybeMerge(f *failure) {
	// In order for two failures to be part of the same slice:

	// 1. The slice of commits which fail must overlap.
	if g.failing.Overlap(f.failing).Empty() {
		return
	}

	// 2. The slices of commits which possibly caused the failures
	//    must overlap.
	brokeIn := g.brokeIn.Overlap(f.brokeIn)
	if brokeIn.Empty() {
		return
	}

	// 3. If both failures have been fixed, the slices of commits which
	//    fixed the failures must overlap.
	fixedIn := g.fixedIn.Overlap(f.fixedIn)
	if fixedIn.Empty() {
		// Both failures were fixed but at different commits. No merge.
		if !g.fixedIn.Empty() && !f.fixedIn.Empty() {
			return
		}
		// Otherwise, take the non-empty fixedIn slice, if any.
		if !g.fixedIn.Empty() {
			fixedIn = g.fixedIn
		} else if !f.fixedIn.Empty() {
			fixedIn = f.fixedIn
		}
	}

	// Construct the failing slice.
	failing := slice{
		start: brokeIn.start,
		end:   fixedIn.start,
	}
	if brokeIn.Empty() {
		failing.start = g.failing.start
		if f.failing.start < failing.start {
			failing.start = f.failing.start
		}
	}
	if fixedIn.Empty() {
		failing.end = g.failing.end
		if f.failing.end > failing.end {
			failing.end = f.failing.end
		}
	}

	// 4. The fixedIn slice of one failure cannot be wholly contained within
	//    the failing slice of the other.
	if !g.fixedIn.Empty() && g.fixedIn.Overlap(f.failing).Len() == g.fixedIn.Len() {
		return
	}
	if !f.fixedIn.Empty() && f.fixedIn.Overlap(g.failing).Len() == f.fixedIn.Len() {
		return
	}
	g.Ids = append(g.Ids, f.id)
	sort.Strings(g.Ids)
	g.brokeIn = brokeIn
	g.failing = failing
	g.fixedIn = fixedIn
}

// permuteFailures returns all permutations of the given slice of failures.
func permuteFailures(failures []*failure) [][]*failure {
	idxs := make([]int, 0, len(failures))
	for i := range failures {
		idxs = append(idxs, i)
	}
	permuteIdxs := util.Permute(idxs)
	rv := make([][]*failure, 0, len(permuteIdxs))
	for _, idxPerm := range permuteIdxs {
		fPerm := make([]*failure, 0, len(idxPerm))
		for _, idx := range idxPerm {
			fPerm = append(fPerm, failures[idx])
		}
		rv = append(rv, fPerm)
	}
	return rv
}

// findFailureGroups finds groups of related failures by attempting to merge
// each failure with every other failure.
func findFailureGroups(failures []*failure, commits []string) ([]*FailureGroup, error) {
	groups := map[string]*FailureGroup{}
	for _, fails := range permuteFailures(failures) {
		for _, f1 := range fails {
			g := fromFailure(f1)
			for _, f2 := range fails {
				if f2.id == f1.id {
					continue
				}
				g.maybeMerge(f2)
			}
			g.resolve(commits)

			// Deduplicate by hashing the ids of the component failures.
			h := md5.New()
			for _, id := range g.Ids {
				if _, err := h.Write([]byte(id)); err != nil {
					return nil, err
				}
			}
			groups[string(h.Sum(nil))] = g
		}
	}

	rv := make([]*FailureGroup, 0, len(groups))
	for _, g := range groups {
		if err := g.Valid(); err != nil {
			return nil, err
		}
		rv = append(rv, g)
	}
	return rv, nil
}

// FindFailureGroups pulls tasks and commits from the given time period and
// finds potentially-related groups of failures.
func FindFailureGroups(repoUrl string, repo *repograph.Graph, taskDb db.TaskReader, start, end time.Time) ([]*FailureGroup, error) {
	commits := commitSlices(repo, start, end)
	tasks, err := taskDb.GetTasksFromDateRange(start, end, repoUrl)
	if err != nil {
		return nil, err
	}
	rv := []*FailureGroup{}
	for _, commitSlice := range commits {
		failures, err := findFailures(tasks, commitSlice)
		if err != nil {
			return nil, err
		}
		groups, err := findFailureGroups(failures, commitSlice)
		if err != nil {
			return nil, err
		}
		rv = append(rv, groups...)
	}
	return rv, nil
}
