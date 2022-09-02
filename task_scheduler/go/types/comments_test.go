package types

import (
	"testing"
	"time"

	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestCopyTaskComment(t *testing.T) {
	v := MakeTaskComment(1, 1, 1, 1, time.Now())
	deleted := true
	v.Deleted = &deleted
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyTaskSpecComment(t *testing.T) {
	v := MakeTaskSpecComment(1, 1, 1, time.Now())
	v.Flaky = true
	v.IgnoreFailure = true
	deleted := true
	v.Deleted = &deleted
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyCommitComment(t *testing.T) {
	v := MakeCommitComment(1, 1, 1, time.Now())
	v.IgnoreFailure = true
	deleted := true
	v.Deleted = &deleted
	assertdeep.Copy(t, v, v.Copy())
}

func TestCopyRepoComments(t *testing.T) {
	v := &RepoComments{
		Repo: "r1",
		TaskComments: map[string]map[string][]*TaskComment{
			"c1": {
				"n1": {MakeTaskComment(1, 1, 1, 1, time.Now())},
			},
		},
		TaskSpecComments: map[string][]*TaskSpecComment{
			"n1": {MakeTaskSpecComment(1, 1, 1, time.Now())},
		},
		CommitComments: map[string][]*CommitComment{
			"c1": {MakeCommitComment(1, 1, 1, time.Now())},
		},
	}
	assertdeep.Copy(t, v, v.Copy())
}
