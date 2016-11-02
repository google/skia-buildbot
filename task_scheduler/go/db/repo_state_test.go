package db

import (
	"testing"

	"go.skia.org/infra/go/testutils"
)

func TestCopyPatch(t *testing.T) {
	testutils.SmallTest(t)
	v := Patch{
		Issue:    "1",
		Patchset: "2",
		Server:   "volley.com",
	}
	testutils.AssertCopy(t, v, v.Copy())
}

func TestCopyRepoState(t *testing.T) {
	testutils.SmallTest(t)
	v := RepoState{
		Patch: Patch{
			Issue:    "1",
			Patchset: "2",
			Server:   "volley.com",
		},
		Repo:     "nou.git",
		Revision: "1",
	}
	testutils.AssertCopy(t, v, v.Copy())
}
