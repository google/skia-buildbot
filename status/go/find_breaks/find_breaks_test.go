package find_breaks

import (
	"fmt"
	"sort"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

func assertContiguous(t *testing.T, v util.StringSet) {
	k := v.Keys()
	if len(k) == 0 {
		return
	}
	sort.Strings(k)
	expect := k[0][0]
	for _, s := range k {
		assert.Equal(t, expect, s[0])
		expect++
	}
}

func assertSubset(t *testing.T, v, set util.StringSet) {
	assert.Equal(t, len(v), len(v.Intersect(set)))
}

func assertValid(t *testing.T, f *failure) {
	assertContiguous(t, f.brokeIn)
	assertContiguous(t, f.failing)
	assertContiguous(t, f.fixedIn)
	assertSubset(t, f.brokeIn, f.failing)
	assertContiguous(t, f.failing.Union(f.fixedIn))
}

func TestMerge(t *testing.T) {
	f1 := &failure{
		id:      "f1",
		brokeIn: util.NewStringSet([]string{"a"}),
		failing: util.NewStringSet([]string{"a"}),
		fixedIn: util.NewStringSet([]string{"b"}),
	}
	f2 := &failure{
		id:      "f2",
		brokeIn: util.NewStringSet([]string{"a"}),
		failing: util.NewStringSet([]string{"a"}),
		fixedIn: util.NewStringSet([]string{"b"}),
	}
	f3 := &failure{
		id:      "f3",
		brokeIn: util.NewStringSet([]string{"b"}),
		failing: util.NewStringSet([]string{"b"}),
		fixedIn: util.NewStringSet([]string{"c"}),
	}
	f4 := &failure{
		id:      "f4",
		brokeIn: util.NewStringSet([]string{"a", "b"}),
		failing: util.NewStringSet([]string{"a", "b"}),
		fixedIn: util.NewStringSet([]string{"c"}),
	}
	f5 := &failure{
		id:      "f5",
		brokeIn: util.NewStringSet([]string{"b", "c"}),
		failing: util.NewStringSet([]string{"b", "c"}),
		fixedIn: util.NewStringSet([]string{"d"}),
	}
	f6 := &failure{
		id:      "f6",
		brokeIn: util.NewStringSet([]string{"a", "b"}),
		failing: util.NewStringSet([]string{"a", "b"}),
		fixedIn: util.NewStringSet([]string{"c", "d"}),
	}
	f7 := &failure{
		id:      "f7",
		brokeIn: util.NewStringSet([]string{"a"}),
		failing: util.NewStringSet([]string{"a"}),
		fixedIn: util.NewStringSet([]string{"b", "c", "d"}),
	}
	f8 := &failure{
		id:      "f8",
		brokeIn: util.NewStringSet([]string{"c"}),
		failing: util.NewStringSet([]string{"c"}),
		fixedIn: util.NewStringSet([]string{"d"}),
	}
	f9 := &failure{
		id:      "f9",
		brokeIn: util.NewStringSet([]string{"a", "b"}),
		failing: util.NewStringSet([]string{"a", "b"}),
		fixedIn: util.NewStringSet([]string{}),
	}

	type testCase struct {
		f1     *failure
		f2     *failure
		expect *failure
	}
	tc := []testCase{
		// 0. Very simple equality.
		testCase{
			f1: f1,
			f2: f2,
			expect: &failure{
				brokeIn: util.NewStringSet([]string{"a"}),
				failing: util.NewStringSet([]string{"a"}),
				fixedIn: util.NewStringSet([]string{"b"}),
			},
		},
		// 1. Not equal.
		testCase{
			f1:     f1,
			f2:     f3,
			expect: nil,
		},
		// 2. Overlap, fixes don't match.
		testCase{
			f1:     f4,
			f2:     f5,
			expect: nil,
		},
		// 3. Overlap, fixes match.
		testCase{
			f1: f6,
			f2: f5,
			expect: &failure{
				brokeIn: util.NewStringSet([]string{"b"}),
				failing: util.NewStringSet([]string{"b"}), // TODO(borenet): This should be b, c.
				fixedIn: util.NewStringSet([]string{"d"}),
			},
		},
		// 4. Totally disjoint.
		testCase{
			f1:     f7,
			f2:     f8,
			expect: nil,
		},
		// 5. The fix set of one failure cannot be wholly contained
		//    within the failed set of the other.
		testCase{
			f1:     f1,
			f2:     f9,
			expect: nil,
		},
	}
	for idx, tc := range tc {
		sklog.Errorf("%d", idx)
		assertValid(t, tc.f1)
		assertValid(t, tc.f2)
		m := merge(tc.f1, tc.f2)
		testutils.AssertDeepEqual(t, tc.expect, m)
		if m != nil {
			assertValid(t, m)
		}
	}
}

func TestFindFailureGroups(t *testing.T) {
	f1 := &failure{
		id:      "1",
		brokeIn: util.NewStringSet([]string{"a"}),
		failing: util.NewStringSet([]string{"a"}),
		fixedIn: util.NewStringSet([]string{"b", "c"}),
	}
	f2 := &failure{
		id:      "2",
		brokeIn: util.NewStringSet([]string{"b"}),
		failing: util.NewStringSet([]string{"b"}),
		fixedIn: util.NewStringSet([]string{"c"}),
	}
	f3 := &failure{
		id:      "3",
		brokeIn: util.NewStringSet([]string{"a", "b"}),
		failing: util.NewStringSet([]string{"a", "b"}),
		fixedIn: util.NewStringSet([]string{"c", "d", "e"}),
	}
	f4 := &failure{
		id:      "4",
		brokeIn: util.NewStringSet([]string{"b", "c"}),
		failing: util.NewStringSet([]string{"b", "c"}),
		fixedIn: util.NewStringSet([]string{"d", "e"}),
	}
	f5 := &failure{
		id:      "5",
		brokeIn: util.NewStringSet([]string{"c", "d"}),
		failing: util.NewStringSet([]string{"c", "d"}),
		fixedIn: util.NewStringSet([]string{"e"}),
	}

	count := 1
	check := func(input []*failure, expect [][]string) {
		sklog.Errorf("%d", count)
		count++
		got, err := findFailureGroups(input)
		assert.NoError(t, err)
		sklog.Errorf("got %d failure groups", len(got))
		remaining := make([][]string, len(expect))
		copy(remaining, expect)
		for _, fg := range got {
			sklog.Errorf("%v", fg)
			found := false
			for i, e := range remaining {
				if util.SSliceEqual(e, fg.ids) {
					remaining = append(remaining[:i], remaining[i+1:]...)
					found = true
					break
				}
			}
			assert.True(t, found, fmt.Sprintf("failure group: %v", fg))
		}
		assert.Equal(t, 0, len(remaining))
	}

	check([]*failure{f1, f2}, [][]string{[]string{f1.id}, []string{f2.id}})
	check([]*failure{f2, f3}, [][]string{[]string{f2.id, f3.id}})
	check([]*failure{f3, f4, f5}, [][]string{[]string{f3.id, f4.id}, []string{f4.id, f5.id}})
	check([]*failure{f1, f2, f3, f4, f5}, [][]string{[]string{f1.id, f3.id}, []string{f2.id, f3.id}, []string{f3.id, f4.id}, []string{f4.id, f5.id}})
}
