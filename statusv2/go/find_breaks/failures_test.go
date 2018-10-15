package find_breaks

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

func TestFindFailures(t *testing.T) {
	testutils.SmallTest(t)

	t1 := &db.Task{
		Id:      "t1",
		Status:  db.TASK_STATUS_SUCCESS,
		Commits: []string{"a"},
	}
	t2 := &db.Task{
		Id:      "t2",
		Status:  db.TASK_STATUS_FAILURE,
		Commits: []string{"b", "c"},
	}
	t3 := &db.Task{
		Id:      "t3",
		Status:  db.TASK_STATUS_SUCCESS,
		Commits: []string{"d"},
	}
	t4 := &db.Task{
		Id:      "t4",
		Status:  db.TASK_STATUS_FAILURE,
		Commits: []string{"e"},
	}
	t5 := &db.Task{
		Id:      "t5",
		Status:  db.TASK_STATUS_FAILURE,
		Commits: []string{"f", "g"},
	}
	t6 := &db.Task{
		Id:      "t6",
		Status:  db.TASK_STATUS_MISHAP,
		Commits: []string{"e"},
	}
	t7 := &db.Task{
		Id:      "t7",
		Status:  db.TASK_STATUS_SUCCESS,
		Commits: []string{"f", "g"},
	}
	t8 := &db.Task{
		Id:      "t8",
		Status:  db.TASK_STATUS_FAILURE,
		Commits: []string{"d"},
	}
	t9 := &db.Task{
		Id:      "t9",
		Status:  db.TASK_STATUS_SUCCESS,
		Commits: []string{"c"},
	}
	t10 := &db.Task{
		Id:      "t10",
		Status:  db.TASK_STATUS_SUCCESS,
		Commits: []string{"h"},
	}
	t11 := &db.Task{
		Id:      "t11",
		Status:  db.TASK_STATUS_FAILURE,
		Commits: []string{"h"},
	}
	t12 := &db.Task{
		Id:      "t12",
		Status:  db.TASK_STATUS_FAILURE,
		Commits: []string{"g"},
	}
	t13 := &db.Task{
		Id:      "t13",
		Status:  db.TASK_STATUS_MISHAP,
		Commits: []string{"f"},
	}
	t14 := &db.Task{
		Id:      "t14",
		Status:  db.TASK_STATUS_SUCCESS,
		Commits: []string{"g"},
	}

	got, err := findFailuresForSpec([]*db.Task{t1, t2, t3, t4, t5}, []string{"a", "b", "c", "d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(got))

	assert.Equal(t, t2.Id, got[0].id)
	assert.Equal(t, newSlice(1, 3), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 3), got[0].failing)
	assert.Equal(t, newSlice(3, 4), got[0].fixedIn)

	assert.Equal(t, t4.Id, got[1].id)
	assert.Equal(t, newSlice(4, 5), got[1].brokeIn)
	assert.Equal(t, newSlice(4, 7), got[1].failing)
	assert.Equal(t, newSlice(-1, -1), got[1].fixedIn)

	// brokeIn starts at the first commit.
	got, err = findFailuresForSpec([]*db.Task{t2, t3}, []string{"b", "c", "d"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 2), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 2), got[0].failing)
	assert.Equal(t, newSlice(2, 3), got[0].fixedIn)

	// Same thing, fixedIn empty.
	got, err = findFailuresForSpec([]*db.Task{t2}, []string{"b", "c", "d"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 2), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 2), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Same thing, but with a gap at the beginning.
	got, err = findFailuresForSpec([]*db.Task{t2}, []string{"a", "b", "c", "d"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 3), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 3), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Success, mishap, success.
	got, err = findFailuresForSpec([]*db.Task{t3, t6, t7}, []string{"d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(got))

	// Failure, mishap, failure.
	got, err = findFailuresForSpec([]*db.Task{t8, t6, t5}, []string{"d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 1), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 4), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Success, failure, mishap, failure.
	got, err = findFailuresForSpec([]*db.Task{t9, t8, t6, t5}, []string{"c", "d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 2), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 5), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Success, mishap, failure.
	got, err = findFailuresForSpec([]*db.Task{t3, t6, t5}, []string{"d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 4), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 4), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Success, mishap, failure, success.
	got, err = findFailuresForSpec([]*db.Task{t3, t6, t5, t10}, []string{"d", "e", "f", "g", "h"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 4), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 4), got[0].failing)
	assert.Equal(t, newSlice(4, 5), got[0].fixedIn)

	// Success, failure, mishap, extra untested commits.
	got, err = findFailuresForSpec([]*db.Task{t9, t8, t6}, []string{"c", "d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 2), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 2), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Success, failure, mishap, success.
	got, err = findFailuresForSpec([]*db.Task{t9, t8, t6, t7}, []string{"c", "d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 2), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 2), got[0].failing)
	assert.Equal(t, newSlice(2, 5), got[0].fixedIn)

	// Success, mishap, failure, failure.
	got, err = findFailuresForSpec([]*db.Task{t3, t6, t5, t11}, []string{"d", "e", "f", "g", "h"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 4), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 5), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Mishap, failure. The failure could have happened at either task.
	got, err = findFailuresForSpec([]*db.Task{t6, t5}, []string{"e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 3), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 3), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Mishap, failure, success.
	got, err = findFailuresForSpec([]*db.Task{t6, t5, t10}, []string{"e", "f", "g", "h"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 3), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 3), got[0].failing)
	assert.Equal(t, newSlice(3, 4), got[0].fixedIn)

	// Success, gap, success.
	got, err = findFailuresForSpec([]*db.Task{t3, t7}, []string{"d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(got))

	// Success, mishap, mishap, failure.
	got, err = findFailuresForSpec([]*db.Task{t3, t6, t13, t12}, []string{"d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 4), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 4), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Failure, mishap, mishap, success.
	got, err = findFailuresForSpec([]*db.Task{t8, t6, t13, t14}, []string{"d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 1), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 1), got[0].failing)
	assert.Equal(t, newSlice(1, 4), got[0].fixedIn)

	// Failure, gap, failure.
	got, err = findFailuresForSpec([]*db.Task{t8, t5}, []string{"d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 1), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 4), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Success, failure, gap, failure.
	got, err = findFailuresForSpec([]*db.Task{t9, t8, t5}, []string{"c", "d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 2), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 5), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Success, gap, failure.
	got, err = findFailuresForSpec([]*db.Task{t3, t5}, []string{"d", "e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 4), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 4), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Success, gap, failure, success.
	got, err = findFailuresForSpec([]*db.Task{t3, t5, t10}, []string{"d", "e", "f", "g", "h"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 4), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 4), got[0].failing)
	assert.Equal(t, newSlice(4, 5), got[0].fixedIn)

	// Success, gap, failure, failure.
	got, err = findFailuresForSpec([]*db.Task{t3, t5, t11}, []string{"d", "e", "f", "g", "h"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(1, 4), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 5), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Gap, failure.
	got, err = findFailuresForSpec([]*db.Task{t5}, []string{"e", "f", "g"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 3), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 3), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Gap, failure, success.
	got, err = findFailuresForSpec([]*db.Task{t5, t10}, []string{"e", "f", "g", "h"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(0, 3), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 3), got[0].failing)
	assert.Equal(t, newSlice(3, 4), got[0].fixedIn)
}
