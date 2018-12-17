package memory

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/modified"
	"go.skia.org/infra/task_scheduler/go/types"
)

// TestCommentBox checks that CommentBox correctly implements CommentDB.
func TestCommentBox(t *testing.T) {
	testutils.SmallTest(t)
	db.TestCommentDB(t, &CommentBox{ModifiedComments: &modified.ModifiedCommentsImpl{}})
}

// TestCommentBoxWithPersistence checks that NewCommentBoxWithPersistence can be
// initialized with a persisted map and will correctly write changes to the
// provided writer.
func TestCommentBoxWithPersistence(t *testing.T) {
	testutils.SmallTest(t)
	expected := map[string]*types.RepoComments{}
	callCount := 0
	testWriter := func(actual map[string]*types.RepoComments) error {
		callCount++
		deepequal.AssertDeepEqual(t, expected, actual)
		return nil
	}

	d := NewCommentBoxWithPersistence(nil, nil, testWriter)

	now := time.Now()

	assert.Equal(t, 0, callCount)

	// Add some comments.
	tc1 := types.MakeTaskComment(1, 1, 1, 1, now)
	r1 := tc1.Repo
	expected[r1] = &types.RepoComments{
		Repo:             r1,
		TaskComments:     map[string]map[string][]*types.TaskComment{"c1": {"n1": {tc1}}},
		TaskSpecComments: map[string][]*types.TaskSpecComment{},
		CommitComments:   map[string][]*types.CommitComment{},
	}
	assert.NoError(t, d.PutTaskComment(tc1))

	tc2 := types.MakeTaskComment(2, 1, 1, 1, now.Add(2*time.Second))
	expected[r1].TaskComments["c1"]["n1"] = []*types.TaskComment{tc1, tc2}
	assert.NoError(t, d.PutTaskComment(tc2))

	tc3 := types.MakeTaskComment(3, 1, 1, 1, now.Add(time.Second))
	expected[r1].TaskComments["c1"]["n1"] = []*types.TaskComment{tc1, tc3, tc2}
	assert.NoError(t, d.PutTaskComment(tc3))

	tc4 := types.MakeTaskComment(4, 1, 1, 2, now)
	expected[r1].TaskComments["c2"] = map[string][]*types.TaskComment{"n1": {tc4}}
	assert.NoError(t, d.PutTaskComment(tc4))

	tc5 := types.MakeTaskComment(5, 1, 2, 2, now)
	expected[r1].TaskComments["c2"]["n2"] = []*types.TaskComment{tc5}
	assert.NoError(t, d.PutTaskComment(tc5))

	tc6 := types.MakeTaskComment(6, 2, 3, 3, now)
	r2 := tc6.Repo
	expected[r2] = &types.RepoComments{
		Repo:             r2,
		TaskComments:     map[string]map[string][]*types.TaskComment{"c3": {"n3": {tc6.Copy()}}},
		TaskSpecComments: map[string][]*types.TaskSpecComment{},
		CommitComments:   map[string][]*types.CommitComment{},
	}
	assert.NoError(t, d.PutTaskComment(tc6))

	tc6copy := tc6.Copy() // Adding identical comment should be ignored.
	assert.NoError(t, d.PutTaskComment(tc6copy))
	tc6.Message = "modifying after Put shouldn't affect stored comment"

	assert.True(t, callCount >= 6)

	sc1 := types.MakeTaskSpecComment(1, 1, 1, now)
	expected[r1].TaskSpecComments["n1"] = []*types.TaskSpecComment{sc1}
	assert.NoError(t, d.PutTaskSpecComment(sc1))

	cc1 := types.MakeCommitComment(1, 1, 1, now)
	expected[r1].CommitComments["c1"] = []*types.CommitComment{cc1}
	assert.NoError(t, d.PutCommitComment(cc1))

	assert.True(t, callCount >= 8)
	callCount = 0

	// Check that if there's an error adding, testWriter is not called.
	tc1different := tc1.Copy()
	tc1different.Message = "not the same"
	assert.True(t, db.IsAlreadyExists(d.PutTaskComment(tc1different)))
	sc1different := sc1.Copy()
	sc1different.Message = "not the same"
	assert.True(t, db.IsAlreadyExists(d.PutTaskSpecComment(sc1different)))
	cc1different := cc1.Copy()
	cc1different.Message = "not the same"
	assert.True(t, db.IsAlreadyExists(d.PutCommitComment(cc1different)))

	assert.Equal(t, 0, callCount)

	// Reload DB from persistent.
	init := map[string]*types.RepoComments{
		r1: expected[r1].Copy(),
		r2: expected[r2].Copy(),
	}
	d = NewCommentBoxWithPersistence(nil, init, testWriter)

	{
		actual, err := d.GetCommentsForRepos([]string{"r0", r1, r2}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		expectedSlice := []*types.RepoComments{
			{Repo: "r0"},
			expected[r1],
			expected[r2],
		}
		deepequal.AssertDeepEqual(t, expectedSlice, actual)
	}

	assert.Equal(t, 0, callCount)

	// Delete some comments.
	expected[r1].TaskComments["c1"]["n1"] = []*types.TaskComment{tc1, tc2}
	assert.NoError(t, d.DeleteTaskComment(tc3))
	expected[r1].TaskSpecComments = map[string][]*types.TaskSpecComment{}
	assert.NoError(t, d.DeleteTaskSpecComment(sc1))
	expected[r1].CommitComments = map[string][]*types.CommitComment{}
	assert.NoError(t, d.DeleteCommitComment(cc1))

	assert.Equal(t, 3, callCount)

	// Delete of nonexistent task should succeed.
	assert.NoError(t, d.DeleteTaskComment(types.MakeTaskComment(99, 1, 1, 1, now.Add(99*time.Second))))
	assert.NoError(t, d.DeleteTaskComment(types.MakeTaskComment(99, 1, 1, 99, now)))
	assert.NoError(t, d.DeleteTaskComment(types.MakeTaskComment(99, 1, 99, 1, now)))
	assert.NoError(t, d.DeleteTaskComment(types.MakeTaskComment(99, 99, 1, 1, now)))
	assert.NoError(t, d.DeleteTaskSpecComment(types.MakeTaskSpecComment(99, 1, 1, now.Add(99*time.Second))))
	assert.NoError(t, d.DeleteTaskSpecComment(types.MakeTaskSpecComment(99, 1, 99, now)))
	assert.NoError(t, d.DeleteTaskSpecComment(types.MakeTaskSpecComment(99, 99, 1, now)))
	assert.NoError(t, d.DeleteCommitComment(types.MakeCommitComment(99, 1, 1, now.Add(99*time.Second))))
	assert.NoError(t, d.DeleteCommitComment(types.MakeCommitComment(99, 1, 99, now)))
	assert.NoError(t, d.DeleteCommitComment(types.MakeCommitComment(99, 99, 1, now)))

	{
		actual, err := d.GetCommentsForRepos([]string{"r0", r1, r2}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		expectedSlice := []*types.RepoComments{
			{Repo: "r0"},
			expected[r1],
			expected[r2],
		}
		deepequal.AssertDeepEqual(t, expectedSlice, actual)
	}

	// Reload DB from persistent again.
	init = map[string]*types.RepoComments{
		r1: expected[r1].Copy(),
		r2: expected[r2].Copy(),
	}
	d = NewCommentBoxWithPersistence(nil, init, testWriter)

	{
		actual, err := d.GetCommentsForRepos([]string{"r0", r1, r2}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		expectedSlice := []*types.RepoComments{
			{Repo: "r0"},
			expected[r1],
			expected[r2],
		}
		deepequal.AssertDeepEqual(t, expectedSlice, actual)
	}
}

// TestCommentBoxWithPersistenceError verifies that when the writer passed to
// NewCommentBoxWithPersistence returns an error, the modification does not take
// effect.
func TestCommentBoxWithPersistenceError(t *testing.T) {
	testutils.SmallTest(t)
	callCount := 0
	var injectedError error = nil
	testWriter := func(actual map[string]*types.RepoComments) error {
		callCount++
		return injectedError
	}

	d := NewCommentBoxWithPersistence(nil, nil, testWriter)

	now := time.Now()

	// Add some comments.
	tc1 := types.MakeTaskComment(1, 1, 1, 1, now)
	tc2 := types.MakeTaskComment(2, 1, 1, 1, now.Add(2*time.Second))
	tc3 := types.MakeTaskComment(3, 1, 1, 1, now.Add(time.Second))
	tc4 := types.MakeTaskComment(4, 1, 1, 2, now)
	tc5 := types.MakeTaskComment(5, 1, 2, 2, now)
	tc6 := types.MakeTaskComment(6, 2, 3, 3, now)
	for _, c := range []*types.TaskComment{tc1, tc2, tc3, tc4, tc5, tc6} {
		assert.NoError(t, d.PutTaskComment(c))
	}
	r1 := tc1.Repo
	r2 := tc6.Repo

	sc1 := types.MakeTaskSpecComment(1, 1, 1, now)
	assert.NoError(t, d.PutTaskSpecComment(sc1))

	cc1 := types.MakeCommitComment(1, 1, 1, now)
	assert.NoError(t, d.PutCommitComment(cc1))

	expected := []*types.RepoComments{
		{
			Repo: r1,
			TaskComments: map[string]map[string][]*types.TaskComment{
				"c1": {
					"n1": {tc1, tc3, tc2},
				},
				"c2": {
					"n1": {tc4},
					"n2": {tc5},
				},
			},
			TaskSpecComments: map[string][]*types.TaskSpecComment{
				"n1": {sc1},
			},
			CommitComments: map[string][]*types.CommitComment{
				"c1": {cc1},
			},
		},
		{
			Repo: r2,
			TaskComments: map[string]map[string][]*types.TaskComment{
				"c3": {
					"n3": {tc6},
				},
			},
			TaskSpecComments: map[string][]*types.TaskSpecComment{},
			CommitComments:   map[string][]*types.CommitComment{},
		},
	}

	{
		actual, err := d.GetCommentsForRepos([]string{r1, r2}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, expected, actual)
	}

	callCount = 0

	injectedError = fmt.Errorf("No comments from the peanut gallery.")

	assert.Error(t, d.PutTaskComment(types.MakeTaskComment(99, 1, 1, 1, now.Add(99*time.Second))))
	assert.Error(t, d.PutTaskComment(types.MakeTaskComment(99, 1, 1, 99, now)))
	assert.Error(t, d.PutTaskComment(types.MakeTaskComment(99, 1, 99, 1, now)))
	assert.Error(t, d.PutTaskComment(types.MakeTaskComment(99, 99, 1, 1, now)))
	assert.Error(t, d.PutTaskSpecComment(types.MakeTaskSpecComment(99, 1, 1, now.Add(99*time.Second))))
	assert.Error(t, d.PutTaskSpecComment(types.MakeTaskSpecComment(99, 1, 99, now)))
	assert.Error(t, d.PutTaskSpecComment(types.MakeTaskSpecComment(99, 99, 1, now)))
	assert.Error(t, d.PutCommitComment(types.MakeCommitComment(99, 1, 1, now.Add(99*time.Second))))
	assert.Error(t, d.PutCommitComment(types.MakeCommitComment(99, 1, 99, now)))
	assert.Error(t, d.PutCommitComment(types.MakeCommitComment(99, 99, 1, now)))

	assert.Equal(t, 10, callCount)

	// Assert nothing has changed.
	{
		actual, err := d.GetCommentsForRepos([]string{r1, r2}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, expected, actual)
	}

	assert.Error(t, d.DeleteTaskComment(tc1))
	assert.Error(t, d.DeleteTaskSpecComment(sc1))
	assert.Error(t, d.DeleteCommitComment(cc1))

	// Assert nothing has changed.
	{
		actual, err := d.GetCommentsForRepos([]string{r1, r2}, now.Add(-10000*time.Hour))
		assert.NoError(t, err)
		deepequal.AssertDeepEqual(t, expected, actual)
	}
}
