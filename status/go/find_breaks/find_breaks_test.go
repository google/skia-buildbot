package find_breaks

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

var (
	commits = []string{"a", "b", "c", "d", "e", "f", "g"}
)

func assertSubslice(t *testing.T, v, slice slice) {
	assert.Equal(t, v.Len(), v.Overlap(slice).Len())
}

func assertValid(t *testing.T, f *failure) {
	assert.Equal(t, 0, f.brokeIn.Overlap(f.fixedIn).Len())
	// assert(brokeIn < fixedIn)
	assertSubslice(t, f.brokeIn, f.failing)
}

func TestMerge(t *testing.T) {
	f1 := &failure{
		id:      "f1",
		brokeIn: newSlice(0, 1), // a
		failing: newSlice(0, 1), // a
		fixedIn: newSlice(1, 2), // b
	}
	f2 := &failure{
		id:      "f2",
		brokeIn: newSlice(0, 1), // a
		failing: newSlice(0, 1), // a
		fixedIn: newSlice(1, 2), // b
	}
	f3 := &failure{
		id:      "f3",
		brokeIn: newSlice(1, 2), // b
		failing: newSlice(1, 2), // b
		fixedIn: newSlice(2, 3), // c
	}
	f4 := &failure{
		id:      "f4",
		brokeIn: newSlice(0, 2), // a b
		failing: newSlice(0, 2), // a b
		fixedIn: newSlice(2, 3), // c
	}
	f5 := &failure{
		id:      "f5",
		brokeIn: newSlice(1, 2), // b
		failing: newSlice(1, 3), // b c
		fixedIn: newSlice(3, 4), // d
	}
	f6 := &failure{
		id:      "f6",
		brokeIn: newSlice(0, 2), // a b
		failing: newSlice(0, 2), // a b
		fixedIn: newSlice(2, 4), // c d
	}
	f7 := &failure{
		id:      "f7",
		brokeIn: newSlice(0, 1), // a
		failing: newSlice(0, 1), // a
		fixedIn: newSlice(1, 4), // b c d
	}
	f8 := &failure{
		id:      "f8",
		brokeIn: newSlice(2, 3), // c
		failing: newSlice(2, 3), // c
		fixedIn: newSlice(3, 4), // d
	}
	f9 := &failure{
		id:      "f9",
		brokeIn: newSlice(0, 2),   // a b
		failing: newSlice(0, 2),   // a b
		fixedIn: newSlice(-1, -1), // ?
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
				brokeIn: newSlice(0, 1),
				failing: newSlice(0, 1),
				fixedIn: newSlice(1, 2),
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
				brokeIn: newSlice(1, 2),
				failing: newSlice(1, 3),
				fixedIn: newSlice(3, 4),
			},
		},
		// 4. Totally disjoint.
		testCase{
			f1:     f7,
			f2:     f8,
			expect: nil,
		},
		// 5. The fix slice of one failure cannot be wholly contained
		//    within the failed slice of the other.
		testCase{
			f1:     f1,
			f2:     f9,
			expect: nil,
		},
	}
	for _, tc := range tc {
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
		brokeIn: newSlice(0, 1), // a
		failing: newSlice(0, 1), // a
		fixedIn: newSlice(1, 3), // b c
	}
	f2 := &failure{
		id:      "2",
		brokeIn: newSlice(1, 2), // b
		failing: newSlice(1, 2), // b
		fixedIn: newSlice(2, 3), // c
	}
	f3 := &failure{
		id:      "3",
		brokeIn: newSlice(0, 2), // a b
		failing: newSlice(0, 2), // a b
		fixedIn: newSlice(2, 5), // c d e
	}
	f4 := &failure{
		id:      "4",
		brokeIn: newSlice(1, 3), // b c
		failing: newSlice(1, 3), // b c
		fixedIn: newSlice(3, 5), // d e
	}
	f5 := &failure{
		id:      "5",
		brokeIn: newSlice(2, 4), // c d
		failing: newSlice(2, 4), // c d
		fixedIn: newSlice(4, 5), // e
	}

	check := func(input []*failure, expect [][]string) {
		got, err := findFailureGroups(input)
		assert.NoError(t, err)
		remaining := make([][]string, len(expect))
		copy(remaining, expect)
		for _, fg := range got {
			found := false
			for i, e := range remaining {
				if util.SSliceEqual(e, fg.Ids) {
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
