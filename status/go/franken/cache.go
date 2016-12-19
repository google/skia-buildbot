// Defines BTCache, a Frankenstein cache containing both Builds and Tasks.
package franken

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/status/go/build_cache"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/window"
)

const (
	// Builder names are generated from Task Spec names by appending this suffix.
	TASK_SPEC_BUILDER_SUFFIX = "_NoBuildbot"
	// When no swarming slave has been assigned, we use this for Build.BuildSlave.
	DEFAULT_BUILD_SLAVE = "task_scheduler"
	// Used for Build.Master.
	FAKE_MASTER = "fake_master.task_scheduler"
	// Used for the third element of a property in Build.Properties.
	PROPERTY_SOURCE = "BTCache"
	// MAX_TASKS is the number of tasks we can find via Build.Number via lookup in
	// an LRUCache. If a task falls out of this cache, we can no longer add
	// comments to that task.
	MAX_TASKS = 1000000
	// MAX_COMMENTS is the number of comments we can find via BuildComment.Id or
	// BuilderComment.Id via lookup in an LRUCache. If a comment falls out of this
	// cache, we can no longer delete that comment.
	MAX_COMMENTS = 1000

	// TASK_URL_FMT is a format string for the Swarming task URL. Parameter is
	// task ID.
	TASK_URL_FMT = "https://chromium-swarm.appspot.com/task?id=%s"
	// NOAUTH_TASK_URL_FMT is a format string for the Swarming task URL that is
	// available to users that are not logged in. Parameter is task ID.
	NOAUTH_TASK_URL_FMT = "https://luci-milo.appspot.com/swarming/task/%s"
	// TASK_TRIGGER_URL_FMT is a format string for triggering a Task with task
	// scheduler. Parameters are task spec name and commit hash.
	TASK_TRIGGER_URL_FMT = "https://task-scheduler.skia.org/trigger?submit=true&job=%s&commit=%s"
	// TASKLIST_URL_FMT is a format string for the Swarming tasklist URL.
	// Parameters are a single tag key and value.
	TASKLIST_URL_FMT = "https://chromium-swarm.appspot.com/tasklist?c=name&c=state&c=created_ts&c=duration&c=completed_ts&c=source_revision&f=%s%%3A%s&l=50&s=created_ts%%3Adesc"
	// BOT_DETAIL_URL_FMT is a format string for the Swarming bot detail URL.
	// Parameter is bot name.
	BOT_DETAIL_URL_FMT = "https://chromium-swarm.appspot.com/bot?id=%s"
)

// BTCache is API-compatible with BuildCache, but also includes Tasks.
type BTCache struct {
	// repos, tasks, commentDb, builds, taskNumberCache, and commentIdCache are
	// safe for concurrent use.
	repos     repograph.Map
	tasks     db.TaskCache
	commentDb db.CommentDB
	builds    *build_cache.BuildCache
	// taskNumberCache maps Build.Number for a Build generated from a Task to
	// Task.Id.
	taskNumberCache util.LRUCache
	// commentIdCache maps BuildComment.Id or BuilderComment.Id for a generated
	// comment to the *TaskComment or *TaskSpecComment from which it was
	// generated.
	commentIdCache util.LRUCache
	// mutex protects cachedTaskComments and cachedTaskSpecComments.
	mutex sync.RWMutex
	// cachedCommitComments contains CommitComments.
	cachedCommitComments map[string]map[string][]*buildbot.CommitComment
	// cachedTaskComments contains BuildComments generated from the latest
	// TaskComments.
	// map[TaskComment.Repo][TaskComment.Revision][TaskComment.Name][]BuildComment
	cachedTaskComments map[string]map[string]map[string][]*buildbot.BuildComment
	// cachedTaskSpecComments contains BuilderComments generated from the latest
	// TaskSpecComments. map[BuilderName][]BuilderComment
	cachedTaskSpecComments map[string][]*buildbot.BuilderComment
}

// NewBTCache creates a combined Build and Task cache for the given repos,
// pulling data from the given buildDb and taskDb.
func NewBTCache(repos repograph.Map, buildDb buildbot.DB, taskDb db.RemoteDB) (*BTCache, error) {
	w, err := window.New(build_cache.BUILD_LOADING_PERIOD, 0, nil)
	if err != nil {
		return nil, err
	}
	tasks, err := db.NewTaskCache(taskDb, w)
	if err != nil {
		return nil, err
	}
	builds, err := build_cache.NewBuildCache(buildDb)
	if err != nil {
		return nil, err
	}
	c := &BTCache{
		repos:           repos,
		tasks:           tasks,
		commentDb:       taskDb,
		builds:          builds,
		taskNumberCache: util.NewMemLRUCache(MAX_TASKS),
		commentIdCache:  util.NewMemLRUCache(MAX_COMMENTS),
	}
	if err := c.update(); err != nil {
		return nil, err
	}
	go func() {
		for _ = range time.Tick(time.Minute) {
			if err := c.update(); err != nil {
				glog.Error(err)
			}
		}
	}()
	return c, nil
}

// taskNameToBuilderName generates a Builder name from a TaskSpec name.
func taskNameToBuilderName(name string) string {
	return name + TASK_SPEC_BUILDER_SUFFIX
}

// builderNameToTaskName returns the TaskSpec name if the given Builder name was
// generated from a TaskSpec name. Otherwise returns name. The second return
// value is true if a TaskSpec name, false if a Builder name.
func builderNameToTaskName(name string) (string, bool) {
	if strings.HasSuffix(name, TASK_SPEC_BUILDER_SUFFIX) {
		return name[:len(name)-len(TASK_SPEC_BUILDER_SUFFIX)], true
	} else {
		return name, false
	}
}

// commentId generates a number to use as BuildComment.Id or BuilderComment.Id,
// which will then be retrievable via taskCommentForId or taskSpecCommentForId.
func (c *BTCache) commentId(comment interface{}) int64 {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(comment); err != nil {
		glog.Error(err)
		return -1
	}
	// Although the Id field is int64, we use int32 to avoid issues in Javascript.
	hash := fnv.New32a()
	_, _ = hash.Write(buf.Bytes())
	// Shift down by one to ensure non-negative.
	id := int32(hash.Sum32() >> 1)
	c.commentIdCache.Add(id, comment)
	return int64(id)
}

// taskCommentForId returns the TaskComment that was assigned the given id, or
// error if the comment is too old or not found.
func (c *BTCache) taskCommentForId(id int64) (*db.TaskComment, error) {
	v, ok := c.commentIdCache.Get(int32(id))
	if !ok {
		return nil, fmt.Errorf("Unknown TaskComment id %d", id)
	}
	return v.(*db.TaskComment), nil
}

// taskSpecCommentForId returns the TaskSpecComment that was assigned the given
// id, or error if the comment is too old or not found.
func (c *BTCache) taskSpecCommentForId(id int64) (*db.TaskSpecComment, error) {
	v, ok := c.commentIdCache.Get(int32(id))
	if !ok {
		return nil, fmt.Errorf("Unknown TaskSpecComment id %d", id)
	}
	return v.(*db.TaskSpecComment), nil
}

// update reads updated tasks and comments from the task scheduler DB. (Builds
// are updated automatically by BuildCache.)
func (c *BTCache) update() error {
	if err := c.repos.Update(); err != nil {
		return err
	}
	if err := c.tasks.Update(); err != nil {
		return err
	}
	return c.updateComments()
}

// updateComments reads updated comments from the task scheduler DB. This method
// is separate from update to avoid calling c.repos.Update when adding/deleting
// comments.
func (c *BTCache) updateComments() error {
	repoNames := c.repos.RepoURLs()
	comments, err := c.commentDb.GetCommentsForRepos(repoNames, time.Now().Add(-build_cache.BUILD_LOADING_PERIOD))
	if err != nil {
		return err
	}
	commitComments := make(map[string]map[string][]*buildbot.CommitComment, len(repoNames))
	taskComments := make(map[string]map[string]map[string][]*buildbot.BuildComment, len(repoNames))
	taskSpecComments := map[string][]*buildbot.BuilderComment{}
	for _, rc := range comments {
		for _, comments := range rc.CommitComments {
			for _, comment := range comments {
				commitMap, ok := commitComments[comment.Repo]
				if !ok {
					commitMap = map[string][]*buildbot.CommitComment{}
					commitComments[comment.Repo] = commitMap
				}
				c := &buildbot.CommitComment{
					Id:            comment.Timestamp.UnixNano(),
					Commit:        comment.Revision,
					User:          comment.User,
					Timestamp:     comment.Timestamp,
					IgnoreFailure: comment.IgnoreFailure,
					Message:       comment.Message,
				}
				commitMap[comment.Revision] = append(commitMap[comment.Revision], c)
			}
		}
		for _, m := range rc.TaskComments {
			for _, comments := range m {
				comment := comments[0]
				commitMap, ok := taskComments[comment.Repo]
				if !ok {
					commitMap = map[string]map[string][]*buildbot.BuildComment{}
					taskComments[comment.Repo] = commitMap
				}
				nameMap, ok := commitMap[comment.Revision]
				if !ok {
					nameMap = map[string][]*buildbot.BuildComment{}
					commitMap[comment.Revision] = nameMap
				}
				buildComments := nameMap[comment.Name]
				for _, tc := range comments {
					id := c.commentId(tc)
					buildComments = append(buildComments, &buildbot.BuildComment{
						Id:        id,
						User:      tc.User,
						Timestamp: tc.Timestamp,
						Message:   tc.Message,
					})
				}
				nameMap[comment.Name] = buildComments
			}
		}
		for _, comments := range rc.TaskSpecComments {
			builderName := taskNameToBuilderName(comments[0].Name)
			builderComments := taskSpecComments[builderName]
			for _, tsc := range comments {
				id := c.commentId(tsc)
				builderComments = append(builderComments, &buildbot.BuilderComment{
					Id:            id,
					Builder:       builderName,
					User:          tsc.User,
					Timestamp:     tsc.Timestamp,
					Flaky:         tsc.Flaky,
					IgnoreFailure: tsc.IgnoreFailure,
					Message:       tsc.Message,
				})
			}
			taskSpecComments[builderName] = builderComments
		}
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cachedCommitComments = commitComments
	c.cachedTaskComments = taskComments
	c.cachedTaskSpecComments = taskSpecComments
	return nil
}

// taskIdToBuildNumber returns a unique number to use as Build.Number.
func (c *BTCache) taskIdToBuildNumber(id string) int {
	// Very hacky...
	_, seq, err := local_db.ParseId(id)
	if err != nil {
		glog.Error(err)
		return -1
	}
	num := int(seq)
	c.taskNumberCache.Add(num, id)
	return num
}

// buildNumberToTaskId returns the Task.Id that was assigned the given build
// number, or an error if the task is too old or not found.
func (c *BTCache) buildNumberToTaskId(num int) (string, error) {
	value, ok := c.taskNumberCache.Get(num)
	if !ok {
		return "", fmt.Errorf("Unknown Task %d", num)
	}
	return value.(string), nil
}

// taskToBuild generates a Build representing a Task. Assumes the caller holds
// a lock.
func (c *BTCache) taskToBuild(task *db.Task, loggedIn bool) *buildbot.Build {
	results := buildbot.BUILDBOT_EXCEPTION
	switch task.Status {
	case db.TASK_STATUS_PENDING, db.TASK_STATUS_RUNNING, db.TASK_STATUS_SUCCESS:
		results = buildbot.BUILDBOT_SUCCESS
	case db.TASK_STATUS_FAILURE:
		results = buildbot.BUILDBOT_FAILURE
	case db.TASK_STATUS_MISHAP:
		results = buildbot.BUILDBOT_EXCEPTION
	}

	buildSlave := DEFAULT_BUILD_SLAVE

	taskUrlFmt := NOAUTH_TASK_URL_FMT
	if loggedIn {
		taskUrlFmt = TASK_URL_FMT
	}
	properties := [][]interface{}{
		{"taskURL", fmt.Sprintf(taskUrlFmt, task.SwarmingTaskId), PROPERTY_SOURCE},
		{"taskRetryURL", fmt.Sprintf(TASK_TRIGGER_URL_FMT, task.Name, task.Revision), PROPERTY_SOURCE},
		{"taskSpecTasklistURL", fmt.Sprintf(TASKLIST_URL_FMT, db.SWARMING_TAG_NAME, task.Name), PROPERTY_SOURCE},
	}
	if task.SwarmingBotId != "" {
		buildSlave = task.SwarmingBotId
		properties = append(properties, [][]interface{}{
			{"botTasklistURL", fmt.Sprintf(TASKLIST_URL_FMT, "id", task.SwarmingBotId), PROPERTY_SOURCE},
			{"botDetailURL", fmt.Sprintf(BOT_DETAIL_URL_FMT, task.SwarmingBotId), PROPERTY_SOURCE},
		}...)
	}
	propertiesStr := ""
	if propBytes, err := json.Marshal(properties); err == nil {
		propertiesStr = string(propBytes)
	} else {
		glog.Errorf("Failed to encode properties: %s", err)
	}

	finished := time.Time{}
	if task.Done() {
		finished = task.Finished
	}

	comments := c.cachedTaskComments[task.Repo][task.Revision][task.Name]
	if comments == nil {
		comments = []*buildbot.BuildComment{}
	}

	return &buildbot.Build{
		Builder:       taskNameToBuilderName(task.Name),
		Master:        FAKE_MASTER,
		Number:        c.taskIdToBuildNumber(task.Id),
		BuildSlave:    buildSlave,
		Branch:        "", // No one actually looks at this.
		Commits:       task.Commits,
		GotRevision:   task.Revision,
		Properties:    properties,
		PropertiesStr: propertiesStr,
		Results:       results,
		Steps:         []*buildbot.BuildStep{},
		Started:       task.Started,
		Finished:      finished,
		Comments:      comments,
		Repository:    task.Repo,
	}
}

// GetBuildsForCommits returns the build data and task data (as Builds) for the
// given commits. See also BuildCache.GetBuildsForCommits.
func (c *BTCache) GetBuildsForCommits(repoName string, commits []string, loggedIn bool) (map[string]map[string]*buildbot.BuildSummary, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.getBuildsForCommits(repoName, commits, loggedIn)
}

// getBuildsForCommits returns the build data and task data (as Builds) for the
// given commits. See also BuildCache.GetBuildsForCommits.
func (c *BTCache) getBuildsForCommits(repoName string, commits []string, loggedIn bool) (map[string]map[string]*buildbot.BuildSummary, error) {
	if len(commits) == 0 {
		return map[string]map[string]*buildbot.BuildSummary{}, nil
	}

	buildResult, err := c.builds.GetBuildsForCommits(commits)
	if err != nil {
		return nil, err
	}

	taskResult, err := c.tasks.GetTasksForCommits(repoName, commits)
	if err != nil {
		return nil, err
	}

	for hash, taskMap := range taskResult {
		if len(taskMap) == 0 {
			continue
		}
		buildMap, ok := buildResult[hash]
		if !ok {
			buildMap = map[string]*buildbot.BuildSummary{}
			buildResult[hash] = buildMap
		}
		for name, task := range taskMap {
			builder := taskNameToBuilderName(name)
			buildMap[builder] = c.taskToBuild(task, loggedIn).GetSummary()
		}
	}
	return buildResult, nil
}

// GetBuildsForCommit returns the build data and task data (as Builds) for the
// given commit. See also BuildCache.GetBuildsForCommit.
func (c *BTCache) GetBuildsForCommit(repoName, hash string, loggedIn bool) ([]*buildbot.BuildSummary, error) {
	builds, err := c.GetBuildsForCommits(repoName, []string{hash}, loggedIn)
	if err != nil {
		return nil, err
	}
	rv := make([]*buildbot.BuildSummary, 0, len(builds[hash]))
	for _, b := range builds[hash] {
		rv = append(rv, b)
	}
	return rv, nil
}

// GetBuildsFromDateRange returns builds and tasks (as Builds) within the given
// date range. See also BuildCache.GetBuildsFromDateRange.
func (c *BTCache) GetBuildsFromDateRange(from, to time.Time, loggedIn bool) ([]*buildbot.Build, error) {
	buildResult, err := c.builds.GetBuildsFromDateRange(from, to)
	if err != nil {
		return nil, err
	}
	taskResult, err := c.tasks.GetTasksFromDateRange(from, to)
	if err != nil {
		return nil, err
	}
	// TODO(benjaminwagner): Does anyone care if the return value is sorted?
	for _, task := range taskResult {
		if task.Done() {
			buildResult = append(buildResult, c.taskToBuild(task, loggedIn))
		}
	}
	return buildResult, nil
}

// GetBuildersComments returns comments for all builders and TaskSpecs (as
// BuilderComment). See also BuildCache.getBuildersComments.
func (c *BTCache) GetBuildersComments() map[string][]*buildbot.BuilderComment {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.getBuildersComments()
}

// getBuildersComments returns comments for all builders and TaskSpecs (as
// BuilderComment). See also BuildCache.getBuildersComments.
func (c *BTCache) getBuildersComments() map[string][]*buildbot.BuilderComment {
	buildResult := c.builds.GetBuildersComments()
	for name, comments := range c.cachedTaskSpecComments {
		buildResult[name] = comments
	}
	return buildResult
}

// AddBuilderComment adds a comment for either the given builder or a TaskSpec
// if builder represents a Task name. See also BuildCache.AddBuilderComment.
func (c *BTCache) AddBuilderComment(builder string, comment *buildbot.BuilderComment) error {
	name, isTask := builderNameToTaskName(builder)
	if isTask {
		repo := ""
		for repoName, _ := range c.repos {
			if c.tasks.KnownTaskName(repoName, name) {
				repo = repoName
				break
			}
		}
		if repo == "" {
			return fmt.Errorf("Unknown TaskSpec %q (derived from %q)", name, builder)
		}
		taskSpecComment := &db.TaskSpecComment{
			Repo:          repo,
			Name:          name,
			Timestamp:     comment.Timestamp,
			User:          comment.User,
			Flaky:         comment.Flaky,
			IgnoreFailure: comment.IgnoreFailure,
			Message:       comment.Message,
		}
		if err := c.commentDb.PutTaskSpecComment(taskSpecComment); err != nil {
			return err
		}
		return c.updateComments()
	} else {
		return c.builds.AddBuilderComment(builder, comment)
	}
}

// DeleteBuilderComment deletes the given comment, which could represent either
// a BuilderComment or a TaskSpecComment. See also
// BuildCache.DeleteBuilderComment.
func (c *BTCache) DeleteBuilderComment(builder string, commentId int64) error {
	_, isTask := builderNameToTaskName(builder)
	if isTask {
		taskSpecComment, err := c.taskSpecCommentForId(commentId)
		if err != nil {
			return err
		}
		if err := c.commentDb.DeleteTaskSpecComment(taskSpecComment); err != nil {
			return err
		}
		return c.updateComments()
	} else {
		return c.builds.DeleteBuilderComment(builder, commentId)
	}
}

// AddBuildComment adds the given comment as a TaskComment if builder represents
// a Task name, or as a BuildComment. See also BuildCache.AddBuildComment.
func (c *BTCache) AddBuildComment(master, builder string, number int, comment *buildbot.BuildComment) error {
	name, isTask := builderNameToTaskName(builder)
	if isTask {
		taskId, err := c.buildNumberToTaskId(number)
		if err != nil {
			return err
		}
		task, err := c.tasks.GetTask(taskId)
		if err != nil {
			return err
		}
		if name != task.Name {
			return fmt.Errorf("Inconsistent Task name; expected %q, got %q", task.Name, name)
		}
		taskComment := &db.TaskComment{
			Repo:      task.Repo,
			Revision:  task.Revision,
			Name:      task.Name,
			Timestamp: comment.Timestamp,
			User:      comment.User,
			Message:   comment.Message,
		}
		if err := c.commentDb.PutTaskComment(taskComment); err != nil {
			return err
		}
		return c.updateComments()
	} else {
		return c.builds.AddBuildComment(master, builder, number, comment)
	}
}

// DeleteBuildComment deletes the given comment, which could represent either a
// BuildComment or a TaskComment. See also BuildCache.DeleteBuildComment.
func (c *BTCache) DeleteBuildComment(master, builder string, number int, commentId int64) error {
	_, isTask := builderNameToTaskName(builder)
	if isTask {
		taskComment, err := c.taskCommentForId(commentId)
		if err != nil {
			return err
		}
		if err := c.commentDb.DeleteTaskComment(taskComment); err != nil {
			return err
		}
		return c.updateComments()
	} else {
		return c.builds.DeleteBuildComment(master, builder, number, commentId)
	}
}

// getLastNCommits returns the last N commits in the given repo.
func (c *BTCache) getLastNCommits(r *repograph.Graph, n int) ([]*vcsinfo.LongCommit, error) {
	// Find the last Nth commit on master, which we assume has far more
	// commits than any other branch.
	commit := r.Get("master")
	for i := 0; i < n-1; i++ {
		p := commit.GetParents()
		if len(p) < 1 {
			// Cut short if we've hit the beginning of history.
			break
		}
		commit = p[0]
	}

	// Now find all commits newer than the current commit.
	start := commit.Timestamp
	commits := make([]*repograph.Commit, 0, 2*n)
	if err := r.RecurseAllBranches(func(c *repograph.Commit) (bool, error) {
		if !c.Timestamp.Before(start) {
			commits = append(commits, c)
			return true, nil
		}
		return false, nil
	}); err != nil {
		return nil, err
	}

	// Sort the commits by timestamp, most recent first.
	sort.Sort(repograph.CommitSlice(commits))

	// Return the most-recent N commits.
	rv := make([]*vcsinfo.LongCommit, 0, len(commits))
	for _, c := range commits {
		rv = append(rv, c.LongCommit)
		if len(rv) >= n {
			break
		}
	}
	return rv, nil
}

// CommitsData is a struct used for collecting builds and commits.
type CommitsData struct {
	Comments    map[string][]*buildbot.CommitComment         `json:"comments"`
	Commits     []*vcsinfo.LongCommit                        `json:"commits"`
	BranchHeads []*gitinfo.GitBranch                         `json:"branch_heads"`
	Builds      map[string]map[string]*buildbot.BuildSummary `json:"builds"`
	Builders    map[string][]*buildbot.BuilderComment        `json:"builders"`
}

// GetLastN returns commit and build information for the last N commits in the
// given repo.
func (c *BTCache) GetLastN(repo string, n int, loggedIn bool) (*CommitsData, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	r, ok := c.repos[repo]
	if !ok {
		return nil, fmt.Errorf("No such repo: %s", repo)
	}

	commits, err := c.getLastNCommits(r, n)
	if err != nil {
		return nil, err
	}

	commitComments := c.cachedCommitComments[repo]
	if commitComments == nil {
		commitComments = map[string][]*buildbot.CommitComment{}
	}

	branches := r.Branches()
	branchHeads := make([]*gitinfo.GitBranch, 0, len(branches))
	for _, b := range branches {
		branchHeads = append(branchHeads, &gitinfo.GitBranch{
			Name: b,
			Head: r.Get(b).Hash,
		})
	}

	hashes := make([]string, 0, len(commits))
	for _, c := range commits {
		hashes = append(hashes, c.Hash)
	}
	builds, err := c.getBuildsForCommits(repo, hashes, loggedIn)
	if err != nil {
		return nil, err
	}

	builders := c.getBuildersComments()
	return &CommitsData{
		Comments:    commitComments,
		Commits:     commits,
		BranchHeads: branchHeads,
		Builds:      builds,
		Builders:    builders,
	}, nil
}

// AddCommitComment adds a CommitComment.
func (c *BTCache) AddCommitComment(repo string, comment *buildbot.CommitComment) error {
	// Truncate the timestamp to milliseconds.
	ts := comment.Timestamp.Round(time.Millisecond)
	if err := c.commentDb.PutCommitComment(&db.CommitComment{
		Repo:          repo,
		Revision:      comment.Commit,
		Timestamp:     ts,
		User:          comment.User,
		IgnoreFailure: comment.IgnoreFailure,
		Message:       comment.Message,
	}); err != nil {
		return err
	}
	return c.updateComments()
}

// DeleteCommitComment deletes a CommitComment.
func (c *BTCache) DeleteCommitComment(repo, commit string, id int64) error {
	ts := time.Unix(0, id)
	comment := &db.CommitComment{
		Repo:      repo,
		Revision:  commit,
		Timestamp: ts,
	}
	if err := c.commentDb.DeleteCommitComment(comment); err != nil {
		return err
	}
	return c.updateComments()
}

// GetTaskCache returns the underlying db.TaskCache.
func (c *BTCache) GetTaskCache() db.TaskCache {
	return c.tasks
}
