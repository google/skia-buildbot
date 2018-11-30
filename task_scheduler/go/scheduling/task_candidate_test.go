package scheduling

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestCopyTaskCandidate(t *testing.T) {
	testutils.SmallTest(t)
	v := &taskCandidate{
		Attempt:            3,
		BuildbucketBuildId: 8888,
		Commits:            []string{"a", "b"},
		IsolatedInput:      "lonely-parameter",
		IsolatedHashes:     []string{"browns"},
		Jobs: jobSet(&types.Job{
			Id: "dummy",
		}),
		ParentTaskIds:  []string{"38", "39", "40"},
		RetryOf:        "41",
		Score:          99,
		StealingFromId: "rich",
		TaskKey: types.TaskKey{
			RepoState: types.RepoState{
				Repo:     "nou.git",
				Revision: "1",
			},
			Name: "Build",
		},
		TaskSpec: &specs.TaskSpec{
			Isolate: "confine",
		},
	}
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestTaskCandidateId(t *testing.T) {
	testutils.SmallTest(t)
	t1 := makeTaskCandidate("task1", []string{"k:v"})
	t1.Repo = "Myrepo"
	t1.Revision = "abc123"
	t1.ForcedJobId = "someID"
	id1 := t1.MakeId()
	k1, err := parseId(id1)
	assert.NoError(t, err)
	assert.Equal(t, t1.TaskKey, k1)

	// ForcedJobId is allowed to be empty.
	t1.ForcedJobId = ""
	id1 = t1.MakeId()
	k1, err = parseId(id1)
	assert.NoError(t, err)
	assert.Equal(t, t1.TaskKey, k1)

	// Test a try job.
	t1.Server = "https://my-patch.com"
	t1.Issue = "10101"
	t1.Patchset = "42"
	id1 = t1.MakeId()
	k1, err = parseId(id1)
	assert.NoError(t, err)
	assert.Equal(t, t1.TaskKey, k1)

	badIds := []string{
		"",
		"taskCandidate|a",
		"taskCandidate|a|b||ab",
		"20160831T000018.497703717Z_000000000000015b",
	}
	for _, id := range badIds {
		_, err := parseId(id)
		assert.Error(t, err)
	}
}

func TestReplaceVar(t *testing.T) {
	testutils.SmallTest(t)
	c := makeTaskCandidate("c", []string{"k:v"})
	c.Repo = "my-repo"
	c.Revision = "abc123"
	c.Name = "my-task"
	dummyId := "id123"
	assert.Equal(t, "", replaceVars(c, "", dummyId))
	assert.Equal(t, "my-repo", replaceVars(c, "<(REPO)", dummyId))
	assert.Equal(t, "my-task", replaceVars(c, "<(TASK_NAME)", dummyId))
	assert.Equal(t, "abc123", replaceVars(c, "<(REVISION)", dummyId))
	assert.Equal(t, "<(REVISION", replaceVars(c, "<(REVISION", dummyId))
	assert.Equal(t, "my-repo_my-task_abc123", replaceVars(c, "<(REPO)_<(TASK_NAME)_<(REVISION)", dummyId))
	assert.Equal(t, dummyId, replaceVars(c, "<(TASK_ID)", dummyId))
	assert.Equal(t, "", replaceVars(c, "<(PATCH_REF)", dummyId))

	c.Issue = "12345"
	c.Patchset = "3"
	c.Server = "https://server"
	assert.Equal(t, "refs/changes/45/12345/3", replaceVars(c, "<(PATCH_REF)", dummyId))
}
