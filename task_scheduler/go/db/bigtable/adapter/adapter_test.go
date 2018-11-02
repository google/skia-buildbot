package adapter

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/pborman/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/git/repograph"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/bigtable"
)

func TestMain(m *testing.M) {
	db.AssertDeepEqual = deepequal.AssertDeepEqual
	os.Exit(m.Run())
}

func setup(t *testing.T) (db.DB, string, []string, func()) {
	testutils.LargeTest(t)

	project := "test-project"
	// The BigTable emulator persists across tests, so use a different
	// instance for each test to ensure that we start from a clean slate.
	instance := fmt.Sprintf("ts-bigtable-adapter-test-%s", uuid.New())
	assert.NoError(t, bt.InitBigtable(project, instance, bigtable.TABLE_CONFIG))

	ctx := context.Background()
	gb := git_testutils.GitInit(t, ctx)
	commits := git_testutils.GitSetup(ctx, gb)
	commits = append(commits, gb.CommitGen(ctx, "file"))
	commits = append(commits, gb.CommitGen(ctx, "file"))
	commits = append(commits, gb.CommitGen(ctx, "file"))
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	repos, err := repograph.NewMap(ctx, []string{gb.RepoUrl()}, wd)
	assert.NoError(t, err)
	assert.NoError(t, repos.Update(ctx))

	d, err := NewAdapter(context.Background(), project, instance, nil, repos)
	assert.NoError(t, err)
	return d, gb.RepoUrl(), commits, func() {
		testutils.AssertCloses(t, d)
		testutils.RemoveAll(t, wd)
	}
}

func TestBigTableDBTaskDB(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDB(t, d, repo, commits)
}

func TestBigTableDBTaskDBTooManyUsers(t *testing.T) {
	d, _, _, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBTooManyUsers(t, d)
}

func TestBigTableDBTaskDBConcurrentUpdate(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBConcurrentUpdate(t, d, repo, commits)
}

func TestBigTableDBTaskDBUpdateTasksWithRetries(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestUpdateTasksWithRetries(t, d, repo, commits)
}

func TestBigTableDBTaskDBGetTasksFromDateRangeByRepo(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBGetTasksFromDateRangeByRepo(t, d, repo, commits)
}

func TestBigTableDBTaskDBGetTasksFromWindow(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestTaskDBGetTasksFromWindow(t, d, repo, commits)
}

func TestBigTableDBJobDB(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestJobDB(t, d, repo, commits)
}

func TestBigTableDBJobDBTooManyUsers(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestJobDBTooManyUsers(t, d, repo, commits)
}

func TestBigTableDBJobDBConcurrentUpdate(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestJobDBConcurrentUpdate(t, d, repo, commits)
}

func TestBigTableDBJobDBUpdateJobsWithRetries(t *testing.T) {
	d, repo, commits, cleanup := setup(t)
	defer cleanup()
	db.TestUpdateJobsWithRetries(t, d, repo, commits)
}

func TestBigTableDBCommentDB(t *testing.T) {
	d, _, _, cleanup := setup(t)
	defer cleanup()
	db.TestCommentDB(t, d)
}
