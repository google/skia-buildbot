package bigtable

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/task_scheduler/go/db"
	"golang.org/x/oauth2"
)

// Adapter is a struct used to adapt the new bigtableDB to the old db.DB interface.
type adapter struct {
	*bigtableDB
	repos repograph.Map
	db.ModifiedJobs
	db.ModifiedTasks
}

// NewAdapter returns an adapter which wraps a bigtableDB instance to implement
// the old db.DB interface.
func NewAdapter(ctx context.Context, project, instance string, ts oauth2.TokenSource, repos repograph.Map) (db.DBCloser, error) {
	d, err := NewBigTableDB(ctx, project, instance, ts)
	if err != nil {
		return nil, err
	}
	return &adapter{
		bigtableDB: d,
		repos:      repos,
	}, nil
}

// See documentation for db.TaskReader interface.
func (a *adapter) GetTaskById(id string) (*db.Task, error) {
	return a.bigtableDB.GetTaskById(context.Background(), id)
}

// getCommitPrefixesForRepo is a helper which retrieves all commits from the
// repo in a given time range and generates row key prefixes for them.
func getCommitPrefixesForRepo(repoUrl string, graph *repograph.Graph, start, end time.Time) ([]string, error) {
	shortRepo := common.REPO_PROJECT_MAPPING[repoUrl]
	prefixes := make([]string, 0, 128)
	commits, err := graph.GetCommitsNewerThan(start)
	if err != nil {
		return nil, fmt.Errorf("Failed to obtain commit list: %s", err)
	}
	for _, commit := range commits {
		if commit.Timestamp.After(end) {
			break
		}
		prefixes = append(prefixes, fmt.Sprintf("%s-%s", commit.Hash, shortRepo))
	}
	return prefixes, nil
}

// getCommitPrefixes is a helper which retrieves all commits for the given repo
// (or all repos if none is provided) in a given time range and generates row
// key prefixes for them.
func getCommitPrefixes(repo string, repos repograph.Map, start, end time.Time) ([]string, error) {
	var prefixes []string
	if repo == "" {
		for repo, graph := range repos {
			repoPrefixes, err := getCommitPrefixesForRepo(repo, graph, start, end)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, repoPrefixes...)
		}
	} else {
		graph := repos[repo]
		var err error
		prefixes, err = getCommitPrefixesForRepo(repo, graph, start, end)
		if err != nil {
			return nil, err
		}
	}
	return prefixes, nil
}

// See documentation for db.TaskReader interface.
func (a *adapter) GetTasksFromDateRange(start, end time.Time, repo string) ([]*db.Task, error) {
	prefixes, err := getCommitPrefixes(repo, a.repos, start, end)
	if err != nil {
		return nil, err
	}
	// TODO(borenet): This doesn't include tasks for try jobs or manually-
	// triggered jobs!
	return a.GetTasksWithPrefixes(context.Background(), prefixes)
}

// See documentation for db.TaskDB interface.
func (a *adapter) AssignId(task *db.Task) error {
	return a.bigtableDB.AssignTaskId(context.Background(), task)
}

// See documentation for db.TaskDB interface.
func (a *adapter) PutTask(task *db.Task) error {
	if err := a.bigtableDB.PutTask(context.Background(), task, time.Now()); err != nil {
		return err
	}
	a.TrackModifiedTask(task)
	return nil
}

// See documentation for db.TaskDB interface.
func (a *adapter) PutTasks(tasks []*db.Task) error {
	if err := a.bigtableDB.PutTasks(context.Background(), tasks, time.Now()); err != nil {
		return err
	}
	for _, task := range tasks {
		a.TrackModifiedTask(task)
	}
	return nil
}

// See documentation for db.JobReader interface.
func (a *adapter) GetJobById(id string) (*db.Job, error) {
	return a.bigtableDB.GetJobById(context.Background(), id)
}

// See documentation for db.JobReader interface.
func (a *adapter) GetJobsFromDateRange(start, end time.Time) ([]*db.Job, error) {
	prefixes, err := getCommitPrefixes("", a.repos, start, end)
	if err != nil {
		return nil, err
	}
	// TODO(borenet): This doesn't include try jobs or manually-triggered
	// jobs!
	return a.GetJobsWithPrefixes(context.Background(), prefixes)
}

// See documentation for db.JobDB interface.
func (a *adapter) PutJob(job *db.Job) error {
	if err := a.bigtableDB.PutJob(context.Background(), job, time.Now()); err != nil {
		return err
	}
	a.TrackModifiedJob(job)
	return nil
}

// See documentation for db.JobDB interface.
func (a *adapter) PutJobs(jobs []*db.Job) error {
	if err := a.bigtableDB.PutJobs(context.Background(), jobs, time.Now()); err != nil {
		return err
	}
	for _, job := range jobs {
		a.TrackModifiedJob(job)
	}
	return nil
}

// See documentation for db.CommentDB interface.
func (a *adapter) GetCommentsForRepos(repos []string, from time.Time) ([]*db.RepoComments, error) {
	return nil, fmt.Errorf("TODO")
}

// See documentation for db.CommentDB interface.
func (a *adapter) PutTaskComment(c *db.TaskComment) error {
	return fmt.Errorf("TODO")
}

// See documentation for db.CommentDB interface.
func (a *adapter) DeleteTaskComment(c *db.TaskComment) error {
	return fmt.Errorf("TODO")
}

// See documentation for db.CommentDB interface.
func (a *adapter) PutTaskSpecComment(c *db.TaskSpecComment) error {
	return fmt.Errorf("TODO")
}

// See documentation for db.CommentDB interface.
func (a *adapter) DeleteTaskSpecComment(c *db.TaskSpecComment) error {
	return fmt.Errorf("TODO")
}

// See documentation for db.CommentDB interface.
func (a *adapter) PutCommitComment(c *db.CommitComment) error {
	return fmt.Errorf("TODO")
}

// See documentation for db.CommentDB interface.
func (a *adapter) DeleteCommitComment(c *db.CommitComment) error {
	return fmt.Errorf("TODO")
}
