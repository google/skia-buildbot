package find_breaks

import (
	"testing"

	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/task_scheduler/go/db"

	assert "github.com/stretchr/testify/require"
)

func TestFindFailures(t *testing.T) {
	a := commit("a")
	b := commit("b")
	c := commit("c")
	d := commit("d")
	e := commit("e")
	f := commit("f")
	g := commit("g")

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
	got := findFailuresForSpec([]*db.Task{t1, t2, t3, t4, t5}, []*repograph.Commit{a, b, c, d, e, f, g})
	assert.Equal(t, 2, len(got))

	assert.Equal(t, t2.Id, got[0].id)
	assert.Equal(t, newSlice(1, 3), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 3), got[0].failing)
	assert.Equal(t, newSlice(3, 4), got[0].fixedIn)

	assert.Equal(t, t4.Id, got[1].id)
	assert.Equal(t, newSlice(4, 5), got[1].brokeIn)
	assert.Equal(t, newSlice(4, 7), got[1].failing)
	assert.Equal(t, newSlice(-1, -1), got[1].fixedIn)

	// brokeIn should be empty if the very first task is a failure.
	got = findFailuresForSpec([]*db.Task{t2, t3}, []*repograph.Commit{b, c, d})
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(-1, -1), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 2), got[0].failing)
	assert.Equal(t, newSlice(2, 3), got[0].fixedIn)

	// brokeIn and fixedIn empty.
	got = findFailuresForSpec([]*db.Task{t2}, []*repograph.Commit{b, c, d})
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(-1, -1), got[0].brokeIn)
	assert.Equal(t, newSlice(0, 2), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)

	// Same thing, but with a gap at the beginning.
	got = findFailuresForSpec([]*db.Task{t2}, []*repograph.Commit{a, b, c, d})
	assert.Equal(t, 1, len(got))
	assert.Equal(t, newSlice(-1, -1), got[0].brokeIn)
	assert.Equal(t, newSlice(1, 3), got[0].failing)
	assert.Equal(t, newSlice(-1, -1), got[0].fixedIn)
}
