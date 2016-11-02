package git

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func setup(t *testing.T) (*git_testutils.GitBuilder, []string) {
	// Create a local git repo to play with.
	g := git_testutils.GitInit(t)
	commits := make([]string, 10)
	for i := 0; i < 10; i++ {
		commits[9-i] = g.CommitGenMsg("somefile", fmt.Sprintf("Commit Title #%d\n\nCommit Body #%d", i, i))
	}
	return g, commits
}

func TestRepo(t *testing.T) {
	gb, commits := setup(t)
	defer gb.Cleanup()

	tmp, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp)

	r, err := NewRepo(gb.Dir(), tmp)
	assert.NoError(t, err)

	// Verify that we can run git commands.
	_, err = r.Git("branch")
	assert.NoError(t, err)

	// Verify that we don't have a working copy.
	_, err = r.Git("status")
	assert.Error(t, err)
	_, err = r.Git("checkout", "master")
	assert.Error(t, err)

	// Log.
	gotCommits, err := r.RevList("master")
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, commits, gotCommits)

	// Add a commit on the remote.
	c := gb.CommitGen("somefile")

	// Verify that Update() succeeds.
	assert.NoError(t, r.Update())

	// Verify that we got the new commit.
	got, err := r.RevParse(c)
	assert.NoError(t, err)
	assert.Equal(t, c, strings.TrimSpace(got))

	// Verify that we can create a Checkout from the Repo. No need to test
	// the Checkout since that struct has its own tests.
	tmp2, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, tmp2)
	_, err = r.Checkout(tmp2)
	assert.NoError(t, err)
}
