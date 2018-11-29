package types

import (
	"testing"
	"time"

	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestCopyTaskComment(t *testing.T) {
	testutils.SmallTest(t)
	v := MakeTaskComment(1, 1, 1, 1, time.Now())
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestCopyTaskSpecComment(t *testing.T) {
	testutils.SmallTest(t)
	v := MakeTaskSpecComment(1, 1, 1, time.Now())
	v.Flaky = true
	v.IgnoreFailure = true
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestCopyCommitComment(t *testing.T) {
	testutils.SmallTest(t)
	v := MakeCommitComment(1, 1, 1, time.Now())
	v.IgnoreFailure = true
	deepequal.AssertCopy(t, v, v.Copy())
}

func TestCopyRepoComments(t *testing.T) {
	testutils.SmallTest(t)
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
	deepequal.AssertCopy(t, v, v.Copy())
}
