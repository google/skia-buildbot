package build_cache

import (
	"testing"
	"time"

	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/testutils"
)

type testCase struct {
	from   time.Time
	to     time.Time
	expect []string
}

func test(t *testing.T, tree *TimeRangeTree, cases []testCase) {
	for _, tc := range cases {
		actual := tree.GetRange(tc.from, tc.to)
		testutils.AssertDeepEqual(t, tc.expect, actual)
	}
}

func TestTimeRangeTree(t *testing.T) {
	tree := NewTimeRangeTree()

	k1 := time.Unix(1460124300, 0)
	v1 := string(buildbot.MakeBuildID("master1", "builder1", 1))
	tree.Insert(k1, v1)

	k2 := time.Unix(1460124301, 0)
	v2 := string(buildbot.MakeBuildID("master2", "builder2", 2))
	tree.Insert(k2, v2)

	k3 := time.Unix(1460124302, 0)
	v3 := string(buildbot.MakeBuildID("master3", "builder3", 3))
	tree.Insert(k3, v3)

	k4 := time.Unix(1460124303, 0)
	v4 := string(buildbot.MakeBuildID("master4", "builder4", 4))
	tree.Insert(k4, v4)

	k5 := time.Unix(1460124304, 0)
	v5 := string(buildbot.MakeBuildID("master5", "builder5", 5))
	tree.Insert(k5, v5)

	test(t, tree, []testCase{
		// All IDs.
		{
			from:   time.Unix(1460124300, 0),
			to:     time.Unix(1460124305, 0),
			expect: []string{v1, v2, v3, v4, v5},
		},
		// "to" is non-inclusive.
		{
			from:   time.Unix(1460124300, 0),
			to:     time.Unix(1460124304, 0),
			expect: []string{v1, v2, v3, v4},
		},
		// Sub-range.
		{
			from:   time.Unix(1460124302, 0),
			to:     time.Unix(1460124304, 0),
			expect: []string{v3, v4},
		},
	})

	tree.Delete(k3, v3)

	test(t, tree, []testCase{
		// All IDs.
		{
			from:   time.Unix(1460124300, 0),
			to:     time.Unix(1460124305, 0),
			expect: []string{v1, v2, v4, v5},
		},
		// "to" is non-inclusive.
		{
			from:   time.Unix(1460124300, 0),
			to:     time.Unix(1460124304, 0),
			expect: []string{v1, v2, v4},
		},
		// Sub-range.
		{
			from:   time.Unix(1460124302, 0),
			to:     time.Unix(1460124304, 0),
			expect: []string{v4},
		},
	})
}

func TestTimestampCollision(t *testing.T) {
	tree := NewTimeRangeTree()

	k1 := time.Unix(1460124300, 0)
	v1 := string(buildbot.MakeBuildID("master1", "builder1", 1))
	tree.Insert(k1, v1)

	k2 := time.Unix(1460124301, 0)
	v2 := string(buildbot.MakeBuildID("master2", "builder2", 2))
	tree.Insert(k2, v2)

	k3 := time.Unix(1460124302, 0)
	v3 := string(buildbot.MakeBuildID("master3", "builder3", 3))
	tree.Insert(k3, v3)

	k4 := time.Unix(1460124303, 0)
	v4 := string(buildbot.MakeBuildID("master4", "builder4", 4))
	tree.Insert(k4, v4)

	k5 := time.Unix(1460124304, 0)
	v5 := string(buildbot.MakeBuildID("master5", "builder5", 5))
	tree.Insert(k5, v5)

	kDupe := time.Unix(1460124302, 0)
	vDupe := string(buildbot.MakeBuildID("masterDupe", "builderDupe", 99))
	tree.Insert(kDupe, vDupe)

	test(t, tree, []testCase{
		// All IDs.
		{
			from:   time.Unix(1460124300, 0),
			to:     time.Unix(1460124305, 0),
			expect: []string{v1, v2, v3, vDupe, v4, v5},
		},
		// "to" is non-inclusive.
		{
			from:   time.Unix(1460124300, 0),
			to:     time.Unix(1460124304, 0),
			expect: []string{v1, v2, v3, vDupe, v4},
		},
		// Sub-range.
		{
			from:   time.Unix(1460124302, 0),
			to:     time.Unix(1460124304, 0),
			expect: []string{v3, vDupe, v4},
		},
	})

	tree.Delete(kDupe, vDupe)

	test(t, tree, []testCase{
		// All IDs.
		{
			from:   time.Unix(1460124300, 0),
			to:     time.Unix(1460124305, 0),
			expect: []string{v1, v2, v3, v4, v5},
		},
		// "to" is non-inclusive.
		{
			from:   time.Unix(1460124300, 0),
			to:     time.Unix(1460124304, 0),
			expect: []string{v1, v2, v3, v4},
		},
		// Sub-range.
		{
			from:   time.Unix(1460124302, 0),
			to:     time.Unix(1460124304, 0),
			expect: []string{v3, v4},
		},
	})
}
