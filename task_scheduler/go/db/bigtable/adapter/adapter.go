package adapter

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/bigtable"
	"golang.org/x/oauth2"
)

// Adapter is a struct used to adapt the new bigtable.DB to the old db.DB interface.
type adapter struct {
	*bigtable.DB
	db.ModifiedJobs
	db.ModifiedTasks
	repos repograph.Map
}

// NewAdapter returns an adapter which wraps a bigtable.DB instance to implement
// the old db.DB interface.
func NewAdapter(ctx context.Context, project, instance string, ts oauth2.TokenSource, repos repograph.Map) (db.DBCloser, error) {
	d, err := bigtable.NewBigTableDB(ctx, project, instance, ts)
	if err != nil {
		return nil, err
	}
	return &adapter{
		DB:    d,
		repos: repos,
	}, nil
}

// See documentation for db.TaskReader interface.
func (a *adapter) GetTaskById(id string) (*db.Task, error) {
	rv, err := a.DB.GetTaskById(context.Background(), id)
	if err != nil && !db.IsNotFound(err) {
		return nil, err
	}
	return rv, nil
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
		prefixes = append(prefixes, fmt.Sprintf("%s-%s", bigtable.ShortCommit(commit.Hash), shortRepo))
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
		graph, ok := repos[repo]
		if ok {
			var err error
			prefixes, err = getCommitPrefixesForRepo(repo, graph, start, end)
			if err != nil {
				return nil, err
			}
		}
	}
	return prefixes, nil
}

// See documentation for db.TaskReader interface.
func (a *adapter) GetTasksFromDateRange(start, end time.Time, repo string) ([]*db.Task, error) {
	// Git timestamps only have second precision. Widen the range to the
	// nearest second on either end, and we'll filter out the tasks which
	// aren't actually in range after retrieving them from the DB.
	// TODO(borenet): This isn't actually right, since the task could have
	// been created well after the commit landed, but for the transition
	// period it's probably the best we can do.
	startGit := start.Truncate(time.Second)
	endGit := end.Truncate(time.Second).Add(time.Second)
	prefixes, err := getCommitPrefixes(repo, a.repos, startGit, endGit)
	if err != nil {
		return nil, err
	}
	// TODO(borenet): This doesn't include tasks for try jobs or manually-
	// triggered jobs which may have been created in the time range but are
	// for commits outside of the time range.
	tasks, err := a.GetTasksWithPrefixes(context.Background(), prefixes)
	if err != nil {
		return nil, err
	}
	rv := make([]*db.Task, 0, len(tasks))
	for _, task := range tasks {
		if !start.After(task.Created) && task.Created.Before(end) {
			rv = append(rv, task)
		}
	}
	sort.Sort(db.TaskSlice(rv))
	return rv, nil
}

// See documentation for db.TaskDB interface.
func (a *adapter) AssignId(task *db.Task) error {
	return a.DB.AssignTaskId(context.Background(), task)
}

// See documentation for db.TaskDB interface.
func (a *adapter) PutTask(task *db.Task) error {
	if task.Id == "" {
		if err := a.DB.AssignTaskId(context.Background(), task); err != nil {
			return fmt.Errorf("Failed to assign task ID: %s", err)
		}
		if err := a.DB.InsertTask(context.Background(), task); err != nil {
			return fmt.Errorf("Failed to insert task: %s", err)
		}
	} else if err := a.DB.UpdateTask(context.Background(), task); err != nil {
		return err
	}
	a.TrackModifiedTask(task)
	return nil
}

// See documentation for db.TaskDB interface.
func (a *adapter) PutTasks(tasks []*db.Task) error {
	insert := make([]*db.Task, 0, len(tasks))
	update := make([]*db.Task, 0, len(tasks))
	for _, task := range tasks {
		if task.Id == "" {
			insert = append(insert, task)
		} else {
			update = append(update, task)
		}
	}
	if len(insert) > 0 {
		if err := a.DB.AssignTaskIds(context.Background(), insert); err != nil {
			return fmt.Errorf("Failed to assign task IDs: %s", err)
		}
		if err := a.DB.InsertTasks(context.Background(), insert); err != nil {
			return fmt.Errorf("Failed to insert tasks: %s", err)
		}
		for _, task := range insert {
			a.TrackModifiedTask(task)
		}
	}
	if len(update) > 0 {
		if err := a.DB.UpdateTasks(context.Background(), update); err != nil {
			return fmt.Errorf("Failed to update tasks: %s", err)
		}
		for _, task := range update {
			a.TrackModifiedTask(task)
		}
	}
	return nil
}

// See documentation for db.JobReader interface.
func (a *adapter) GetJobById(id string) (*db.Job, error) {
	rv, err := a.DB.GetJobById(context.Background(), id)
	if err != nil && !db.IsNotFound(err) {
		return nil, err
	}
	return rv, nil
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
	if job.Id == "" {
		if err := a.DB.AssignJobId(context.Background(), job); err != nil {
			return fmt.Errorf("Failed to assign job ID: %s", err)
		}
		if err := a.DB.InsertJob(context.Background(), job); err != nil {
			return fmt.Errorf("Failed to insert job: %s", err)
		}
	} else if err := a.DB.UpdateJob(context.Background(), job); err != nil {
		return fmt.Errorf("Failed to update job: %s", err)
	}
	a.TrackModifiedJob(job)
	return nil
}

// See documentation for db.JobDB interface.
func (a *adapter) PutJobs(jobs []*db.Job) error {
	insert := make([]*db.Job, 0, len(jobs))
	update := make([]*db.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Id == "" {
			insert = append(insert, job)
		} else {
			update = append(update, job)
		}
	}
	if len(insert) > 0 {
		if err := a.DB.AssignJobIds(context.Background(), insert); err != nil {
			return fmt.Errorf("Failed to assign job IDs: %s", err)
		}
		if err := a.DB.InsertJobs(context.Background(), insert); err != nil {
			return fmt.Errorf("Failed to insert jobs: %s", err)
		}
		for _, job := range insert {
			a.TrackModifiedJob(job)
		}
	}
	if len(update) > 0 {
		if err := a.DB.UpdateJobs(context.Background(), update); err != nil {
			return fmt.Errorf("Failed to update jobs: %s", err)
		}
		for _, job := range update {
			a.TrackModifiedJob(job)
		}
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
