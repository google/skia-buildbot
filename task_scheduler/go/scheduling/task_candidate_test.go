package scheduling

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskCandidateId(t *testing.T) {
	t1 := makeTaskCandidate("task1", []string{"k:v"})
	t1.Repo = "Myrepo"
	t1.Revision = "abc123"
	id1 := t1.MakeId()
	repo1, name1, rev1, err := parseId(id1)
	assert.NoError(t, err)
	assert.Equal(t, t1.Repo, repo1)
	assert.Equal(t, t1.Name, name1)
	assert.Equal(t, t1.Revision, rev1)

	badIds := []string{
		"",
		"taskCandidate|a|b|",
		"20160831T000018.497703717Z_000000000000015b",
	}
	for _, id := range badIds {
		_, _, _, err := parseId(id)
		assert.Error(t, err)
	}
}
