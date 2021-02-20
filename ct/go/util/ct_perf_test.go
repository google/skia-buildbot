package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

const (
	TEST_GROUP_NAME = "test group name"
	TEST_UNIQUE_ID  = "test unique ID"
	TEST_HASH       = "abcde"
)

func TestCommitToSyntheticRepo(t *testing.T) {
	unittest.MediumTest(t)
	ctx := context.Background()

	// Create a test repo.
	gb := git_testutils.GitInit(t, ctx)
	defer gb.Cleanup()

	// Populate it with an initial whitespace file. Below CreateBranchTrackBranch
	// and CheckoutBranch steps do not seem to work without it.
	gb.Add(ctx, "whitespace.txt", " ")
	gb.CommitMsg(ctx, "Initial whitespace commit")
	// Create a branch and check it out, otherwise we can't push
	// to git.DefaultRemoteBranch on this repo.
	gb.CreateBranchTrackBranch(ctx, "somebranch", git.DefaultRemoteBranch)
	gb.CheckoutBranch(ctx, "somebranch")

	// Create tmp dir that gets cleaned up.
	workdir, err := ioutil.TempDir("", "ct_perf_test_commit")
	require.NoError(t, err)
	defer util.RemoveAll(workdir)

	// Create git.Checkout.
	checkout, err := git.NewCheckout(context.Background(), gb.Dir(), workdir)
	require.NoError(t, err)
	err = checkout.Cleanup(ctx)
	require.NoError(t, err)

	// Make sure email and name are correctly set.
	_, err = checkout.Git(ctx, "config", "user.email", "tester@example.com")
	require.NoError(t, err)
	_, err = checkout.Git(ctx, "config", "user.name", "tester@example.com")
	require.NoError(t, err)

	// Commit to the synthetic repo
	gitExec, err := git.Executable(ctx)
	require.NoError(t, err)
	hash, err := commitToSyntheticRepo(ctx, TEST_GROUP_NAME, TEST_UNIQUE_ID, gitExec, checkout)
	require.NoError(t, err)

	// Confirm that the expected commit is there.
	log, err := checkout.Git(ctx, "log", "--pretty=oneline")
	require.NoError(t, err)
	loglines := strings.Split(log, "\n")
	require.Contains(t, loglines[0], hash)
	require.Contains(t, loglines[0], fmt.Sprintf("Commit for %s by %s", TEST_GROUP_NAME, TEST_UNIQUE_ID))
}

func TestConvertCSVToBenchData(t *testing.T) {
	unittest.SmallTest(t)
	testDataDir := testutils.TestDataDir(t)
	pathToTestCSV := filepath.Join(testDataDir, "test.csv")

	perfData, err := convertCSVToBenchData(TEST_HASH, TEST_GROUP_NAME, TEST_UNIQUE_ID, pathToTestCSV)
	require.NoError(t, err)
	require.NotNil(t, perfData)
	require.Equal(t, perfData.Hash, TEST_HASH)
	require.Equal(t, perfData.RunID, TEST_UNIQUE_ID)
	require.Equal(t, perfData.Key["group_name"], TEST_GROUP_NAME)
	require.Len(t, perfData.Results, 2)
	require.Equal(t, perfData.Results["http://www.reuters.com"]["default"]["paint_op_count"], 805.0)
	require.Equal(t, perfData.Results["http://www.reuters.com"]["default"]["rasterize_time (ms)"], 2.449)
	require.Equal(t, perfData.Results["http://www.reuters.com"]["default"]["options"], map[string]int{"page_rank": 480})
	require.Equal(t, perfData.Results["http://www.rediff.com"]["default"]["paint_op_count"], 643.0)
	require.Equal(t, perfData.Results["http://www.rediff.com"]["default"]["rasterize_time (ms)"], 2.894)
	require.Equal(t, perfData.Results["http://www.rediff.com"]["default"]["options"], map[string]int{"page_rank": 490})
}
