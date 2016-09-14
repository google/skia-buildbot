package scheduling

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskCandidateId(t *testing.T) {
	t1 := makeTaskCandidate("task1", []string{"k:v"})
	t1.Repo = "Myrepo"
	t1.Revision = "abc123"
	t1.ForcedJobId = "someID"
	id1 := t1.MakeId()
	repo1, name1, rev1, forcedJobId1, err := parseId(id1)
	assert.NoError(t, err)
	assert.Equal(t, t1.Repo, repo1)
	assert.Equal(t, t1.Name, name1)
	assert.Equal(t, t1.Revision, rev1)
	assert.Equal(t, t1.ForcedJobId, forcedJobId1)

	t1.ForcedJobId = "" // Allowed to be empty.
	id1 = t1.MakeId()
	repo1, name1, rev1, forcedJobId1, err = parseId(id1)
	assert.NoError(t, err)
	assert.Equal(t, t1.Repo, repo1)
	assert.Equal(t, t1.Name, name1)
	assert.Equal(t, t1.Revision, rev1)
	assert.Equal(t, t1.ForcedJobId, forcedJobId1)

	badIds := []string{
		"",
		"taskCandidate|a|b|",
		"taskCandidate|a|b||ab",
		"20160831T000018.497703717Z_000000000000015b",
	}
	for _, id := range badIds {
		_, _, _, _, err := parseId(id)
		assert.Error(t, err)
	}
}

func TestReplaceVar(t *testing.T) {
	c := makeTaskCandidate("c", []string{"k:v"})
	c.Repo = "my-repo"
	c.Revision = "abc123"
	c.Name = "my-task"
	assert.Equal(t, "", replaceVars(c, ""))
	assert.Equal(t, "my-repo", replaceVars(c, "<(REPO)"))
	assert.Equal(t, "my-task", replaceVars(c, "<(TASK_NAME)"))
	assert.Equal(t, "abc123", replaceVars(c, "<(REVISION)"))
	assert.Equal(t, "<(REVISION", replaceVars(c, "<(REVISION"))
	assert.Equal(t, "my-repo_my-task_abc123", replaceVars(c, "<(REPO)_<(TASK_NAME)_<(REVISION)"))
}
