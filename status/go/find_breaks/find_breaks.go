package find_breaks

import (
	"crypto/md5"
	"sort"
	"time"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/task_scheduler/go/db"
)

// FailureGroup represents a group of failures which are likely to be related.
type FailureGroup struct {
	// Ids are the ids of all of the failures in the group.
	Ids []string

	// BrokeIn is the slice of commits which may have caused the failures in
	// the group. May be empty if the failure could have been caused before
	// the current ocmmit window.
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

// merge combines two failures into a third failure or returns nil if the two
// failures cannot be merged.
func merge(f1, f2 *failure) *failure {
	rv := &failure{}

	// In order for two failures to be part of the same slice:

	// 1. The slice of commits which fail must overlap.
	if f1.failing.Overlap(f2.failing).Empty() {
		return nil
	}

	// 2. The intersection of the slices of commits which caused the failures
	//    must be non-empty. If the break might have occurred before the
	//    current commit window, the brokeIn slice is empty; in this case, the
	//    brokeIn slices of both failures must be empty in order to match.
	rv.brokeIn = f1.brokeIn.Overlap(f2.brokeIn)
	if rv.brokeIn.Empty() {
		if !f1.brokeIn.Empty() || !f2.brokeIn.Empty() {
			return nil
		}
	}

	// 3. If both failures have been fixed, the intersection of the slices of
	//    commits which fixed the failures must be non-empty.
	rv.fixedIn = f1.fixedIn.Overlap(f2.fixedIn)
	rv.failing = slice{
		start: rv.brokeIn.start,
		end:   rv.fixedIn.start,
	}
	if !f1.fixedIn.Empty() && !f2.fixedIn.Empty() {
		if rv.fixedIn.Empty() {
			return nil
		}
		// 3a. If both the brokeIn and fixedIn slices match, we have
		//     enough information to stop here.
		return rv
	}

	// 4. The fixedIn slice of one failure cannot be wholly contained within
	//    the failing slice of the other.
	if f1.fixedIn.Overlap(f2.failing).Len() == f1.fixedIn.Len() {
		return nil
	}
	if f2.fixedIn.Overlap(f1.failing).Len() == f2.fixedIn.Len() {
		return nil
	}
	return rv
}

// findFailureGroups finds groups of related failures by attempting to merge
// each failure with every other failure.
// TODO(borenet): This algorithm is both inefficient (n^2) and possibly
// incorrect, since the results may depend on the ordering of the inputs.
// Test heavily to ensure correctness and try to come up with a more efficient
// algorithm.
func findFailureGroups(failures []*failure) ([]*FailureGroup, error) {
	groups := map[string]*FailureGroup{}
	for _, f1 := range failures {
		ids := []string{f1.id}
		merged := &failure{
			brokeIn: f1.brokeIn.Copy(),
			failing: f1.failing.Copy(),
			fixedIn: f1.fixedIn.Copy(),
		}
		for _, f2 := range failures {
			if f2.id == f1.id {
				continue
			}
			m := merge(merged, f2)
			if m == nil {
				continue
			}
			merged = m
			ids = append(ids, f2.id)
		}
		sort.Strings(ids)
		h := md5.New()
		for _, id := range ids {
			if _, err := h.Write([]byte(id)); err != nil {
				return nil, err
			}
		}
		fg := &FailureGroup{
			Ids:     ids,
			brokeIn: merged.brokeIn,
			failing: merged.failing,
			fixedIn: merged.fixedIn,
		}
		groups[string(h.Sum(nil))] = fg
	}

	rv := make([]*FailureGroup, 0, len(groups))
	for _, fg := range groups {
		rv = append(rv, fg)
	}
	return rv, nil
}

func resolveSlice(s slice, commits []*repograph.Commit) []string {
	if s.start < 0 || s.end < 0 {
		return []string{}
	}
	if s.start >= len(commits) && s.end > len(commits) || s.start > s.end {
		return []string{}
	}
	slice := commits[s.start:s.end]
	rv := make([]string, 0, len(slice))
	for _, c := range slice {
		rv = append(rv, c.Hash)
	}
	return rv
}

func resolveFailureGroup(g *FailureGroup, commits []*repograph.Commit) {
	g.BrokeIn = resolveSlice(g.brokeIn, commits)
	g.Failing = resolveSlice(g.failing, commits)
	g.FixedIn = resolveSlice(g.fixedIn, commits)
}

func FindFailureGroups(repo *repograph.Graph, taskDb db.TaskReader, start, end time.Time) ([]*FailureGroup, error) {
	commits := commitSlices(repo, start, end)
	tasks, err := taskDb.GetTasksFromDateRange(start, end)
	if err != nil {
		return nil, err
	}
	rv := []*FailureGroup{}
	for _, commitSlice := range commits {
		failures := findFailures(tasks, commitSlice)
		groups, err := findFailureGroups(failures)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			resolveFailureGroup(g, commitSlice)
			rv = append(rv, g)
		}
	}
	return rv, nil
}

func main() {
	// Find groups of failing tasks which constitute breakages.

	// Start by merging tasks into contiguous slices of successes and failures
	// within a column (task spec). Note that we're assuming all failures
	// within a column have the same cause. A more intelligent system would
	// be able to distinguish *what* failed and only merge the failures if
	// they matched.

	// Then merge columns into breakages. For each column:
	// - Find the slice of commits which may have caused the break.
	// - Find the slice of commits which may have fixed the break.
	//
	// Two columns may be merged if:
	// - The intersection of their break slices is non-empty.
	// - The intersection of their fix slices is non-empty.
	// - The intersection of the break slice of one column and the fix slice of
	//   the other is empty.
	//
	// However, in merging columns we may reduce the size of the break and
	// fix slices. We want this behavior in some sense, because the goal is to
	// narrow down the blame to a single commit. But we need to make sure
	// that we don't include a failure which then causes us to exclude
	// others. For example:
	//
	//           col1  col2  col3
	//            S     S     S
	//            F     S     F
	//            S     F     F
	//            S     S     S
	//
	// In this case, we can merge col1 with col3 or col2 with col3 but not
	// both. We need to make sure that the algorithm gives us both slices:
	// [col1, col3] and [col2, col3].
	//
	// Edge cases. A failure may extend up to the most recent task, or back
	// to before our time window, such that we can't actually determine what
	// the break or fix slice should be. We can handle this by making the
	// break or fix slice empty (or contain a single special value) and let it
	// behave as a wildcard which matches anything.
}
