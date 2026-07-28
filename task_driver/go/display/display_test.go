package display

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/td"
)

func TestTruncateError(t *testing.T) {

	test := func(input, expect string) {
		assert.Equal(t, expect, truncateError(input))
	}

	// Too small to truncate.
	test("", "")
	test("abc", "abc")

	// Max number of lines.
	test(`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20`, `1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20`)

	// Trim final newline.
	test(`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
`, `1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20`)

	// One line is cut off.
	test(`1
2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21`, `...2
3
4
5
6
7
8
9
10
11
12
13
14
15
16
17
18
19
20
21`)
	// Right at the line and char limit.
	test(`01abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
02abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
03abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
04abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
05abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
06abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
07abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
08abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
09abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
10abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
11abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
12abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
13abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
14abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
15abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
16abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
17abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
18abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
19abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
20abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUV`, `01abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
02abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
03abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
04abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
05abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
06abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
07abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
08abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
09abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
10abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
11abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
12abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
13abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
14abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
15abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
16abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
17abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
18abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
19abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
20abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUV`)

	// Just over the char limit.
	test(`01abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
02abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
03abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
04abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
05abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
06abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
07abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
08abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
09abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
10abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
11abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
12abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
13abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
14abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
15abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
16abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
17abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
18abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
19abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
20abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVW`, `...cdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
02abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
03abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
04abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
05abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
06abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
07abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
08abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
09abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
10abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
11abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
12abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
13abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
14abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
15abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
16abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
17abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
18abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
19abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTU
20abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVW`)
}

func TestTaskDriverForDisplay_SortingByIndex(t *testing.T) {
	now := time.Now()

	// Sibling steps with identical started times but different index.
	// We construct them in non-sorted map order and verify they display sorted by index.
	runA := &db.TaskDriverRun{
		TaskId: "task-A",
		Properties: &td.RunProperties{
			Local: true,
		},
		Steps: map[string]*db.Step{
			td.StepIDRoot: {
				Properties: &td.StepProperties{
					Id: td.StepIDRoot,
				},
				Started: now,
			},
			"step-2": {
				Properties: &td.StepProperties{
					Id:     "step-2",
					Parent: td.StepIDRoot,
					Index:  2,
					Name:   "step-2",
				},
				Started: now,
			},
			"step-3": {
				Properties: &td.StepProperties{
					Id:     "step-3",
					Parent: td.StepIDRoot,
					Index:  3,
					Name:   "step-3",
				},
				Started: now,
			},
			"step-1": {
				Properties: &td.StepProperties{
					Id:     "step-1",
					Parent: td.StepIDRoot,
					Index:  1,
					Name:   "step-1",
				},
				Started: now,
			},
		},
	}

	displayA, err := TaskDriverForDisplay(runA)
	require.NoError(t, err)
	require.NotNil(t, displayA)
	require.Len(t, displayA.Steps, 3)
	assert.Equal(t, "step-1", displayA.Steps[0].Id)
	assert.Equal(t, "step-2", displayA.Steps[1].Id)
	assert.Equal(t, "step-3", displayA.Steps[2].Id)
}

func TestTaskDriverForDisplay_LegacySortingByStarted(t *testing.T) {
	now := time.Now()

	// Sibling steps with index = 0 (legacy mode) but different start times.
	// We verify they fall back to sorting by Started timestamp.
	runB := &db.TaskDriverRun{
		TaskId: "task-B",
		Properties: &td.RunProperties{
			Local: true,
		},
		Steps: map[string]*db.Step{
			td.StepIDRoot: {
				Properties: &td.StepProperties{
					Id: td.StepIDRoot,
				},
				Started: now,
			},
			"step-2": {
				Properties: &td.StepProperties{
					Id:     "step-2",
					Parent: td.StepIDRoot,
					Name:   "step-2",
				},
				Started: now.Add(2 * time.Second),
			},
			"step-3": {
				Properties: &td.StepProperties{
					Id:     "step-3",
					Parent: td.StepIDRoot,
					Name:   "step-3",
				},
				Started: now.Add(3 * time.Second),
			},
			"step-1": {
				Properties: &td.StepProperties{
					Id:     "step-1",
					Parent: td.StepIDRoot,
					Name:   "step-1",
				},
				Started: now.Add(1 * time.Second),
			},
		},
	}

	displayB, err := TaskDriverForDisplay(runB)
	require.NoError(t, err)
	require.NotNil(t, displayB)
	require.Len(t, displayB.Steps, 3)
	assert.Equal(t, "step-1", displayB.Steps[0].Id)
	assert.Equal(t, "step-2", displayB.Steps[1].Id)
	assert.Equal(t, "step-3", displayB.Steps[2].Id)
}
