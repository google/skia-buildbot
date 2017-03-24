package find_breaks

import (
	"crypto/md5"
	"sort"

	"go.skia.org/infra/go/sklog"
)

// set is a struct used for slicing subsets of a larger slice while supporting
// set intersection. Union is not defined, since it is not possible to union
// two arbitrary sets while maintaining a contiguous range.
type set struct {
	start int
	end   int
}

func (s set) Intersect(other set) set {
	rv := set{
		start: s.start,
		end:   s.end,
	}
	if other.start > rv.start {
		rv.start = other.start
	}
	if other.end < rv.end {
		rv.end = other.end
	}
	if rv.start >= rv.end {
		rv.start = -1
		rv.end = -1
	}
	return rv
}

func (s set) Len() int {
	return s.end - s.start
}

func (s set) Empty() bool {
	return s.Len() == 0
}

func (s set) Copy() set {
	return set{
		start: s.start,
		end:   s.end,
	}
}

// failure is a struct which represents a failure of a given task spec,
// potentially merged from several failed tasks.
type failure struct {
	// id is provided by the creator as a handle for the failure.
	id string

	// brokeIn is the set of commits which may have caused the failure.
	// Should be empty if the failure might have been caused before the
	// current commit window.
	brokeIn set

	// failing is the set of commits affected by the failure. Must be a
	// superset of brokeIn. Should not extend outside the current commit
	// window.
	failing set

	// fixedIn is the set of commits which may have fixed the failure. May
	// be empty if the failure has not yet been fixed.
	fixedIn set
}

// failureGroup represents a group of failures which are likely to be related.
type failureGroup struct {
	// ids are the ids of all of the failures in the group.
	ids []string

	// brokeIn is the set of commits which may have caused the failures in
	// the group. May be empty if the failure could have been caused before
	// the current ocmmit window.
	brokeIn set

	// failing is the set of commits which are affected by the failures in
	// the group.
	failing set

	// fixedIn is the set of commits which may have fixed the failures in
	// the group. May be empty if the failures have not yet been fixed.
	fixedIn set
}

// merge combines two failures into a third failure or returns nil if the two
// failures cannot be merged.
func merge(f1, f2 *failure) *failure {
	rv := &failure{}

	// In order for two failures to be part of the same set:

	// 1. The set of commits which fail must overlap.
	rv.failing = f1.failing.Intersect(f2.failing)
	if rv.failing.Empty() {
		sklog.Errorf("f1.failing and f2.failing do not intersect")
		return nil
	}

	// 2. The intersection of the sets of commits which caused the failures
	//    must be non-empty. If the break might have occurred before the
	//    current commit window, the brokeIn set is empty; in this case, the
	//    brokeIn sets of both failures must be empty in order to match.
	rv.brokeIn = f1.brokeIn.Intersect(f2.brokeIn)
	if rv.brokeIn.Empty() {
		if !f1.brokeIn.Empty() || !f2.brokeIn.Empty() {
			sklog.Errorf("f1.brokeIn and f2.brokeIn do not intersect")
			return nil
		}
	}

	// 3. If both failures have been fixed, the intersection of the sets of
	//    commits which fixed the failures must be non-empty.
	rv.fixedIn = f1.fixedIn.Intersect(f2.fixedIn)
	if !f1.fixedIn.Empty() && !f2.fixedIn.Empty() {
		if rv.fixedIn.Empty() {
			sklog.Errorf("f1.fixedIn and f2.fixedIn do not intersect")
			return nil
		}
		// 3a. If both the brokeIn and fixedIn sets match, we have
		//     enough information to stop here.
		return rv
	}

	// 4. The fixedIn set of one failure cannot be wholly contained within
	//    the failing set of the other.
	if f1.fixedIn.Intersect(f2.failing).Len() == f1.fixedIn.Len() {
		return nil
	}
	if f2.fixedIn.Intersect(f1.failing).Len() == f2.fixedIn.Len() {
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
func findFailureGroups(failures []*failure) ([]*failureGroup, error) {
	groups := map[string]*failureGroup{}
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
		fg := &failureGroup{
			ids:     ids,
			brokeIn: merged.brokeIn,
			failing: merged.failing,
			fixedIn: merged.fixedIn,
		}
		groups[string(h.Sum(nil))] = fg
	}

	rv := make([]*failureGroup, 0, len(groups))
	for _, fg := range groups {
		rv = append(rv, fg)
	}
	return rv, nil
}

func main() {
	// Find groups of failing tasks which constitute breakages.

	// Start by merging tasks into contiguous sets of successes and failures
	// within a column (task spec). Note that we're assuming all failures
	// within a column have the same cause. A more intelligent system would
	// be able to distinguish *what* failed and only merge the failures if
	// they matched.

	// Then merge columns into breakages. For each column:
	// - Find the set of commits which may have caused the break.
	// - Find the set of commits which may have fixed the break.
	//
	// Two columns may be merged if:
	// - The intersection of their break sets is non-empty.
	// - The intersection of their fix sets is non-empty.
	// - The intersection of the break set of one column and the fix set of
	//   the other is empty.
	//
	// However, in merging columns we may reduce the size of the break and
	// fix sets. We want this behavior in some sense, because the goal is to
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
	// both. We need to make sure that the algorithm gives us both sets:
	// [col1, col3] and [col2, col3].
	//
	// Edge cases. A failure may extend up to the most recent task, or back
	// to before our time window, such that we can't actually determine what
	// the break or fix set should be. We can handle this by making the
	// break or fix set empty (or contain a single special value) and let it
	// behave as a wildcard which matches anything.
}
