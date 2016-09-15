// Defines BTCache, a Frankenstein cache containing both Builds and Tasks.
package franken

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/status/go/build_cache"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
)

const (
	// Builder names are generated from Task Spec names by appending this suffix.
	TASK_BUILDER_SUFFIX = "-NoBuildbot"
	// When no swarming slave has been assigned, we use this for Build.BuildSlave.
	TASK_BUILD_SLAVE = "task_scheduler"
	// Used for Build.Master.
	TASK_MASTER = "client.skia.fake_internal"
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
)

// BTCache is API-compatible with BuildCache, but also includes Tasks.
type BTCache struct {
	// repos, tasks, commentDb, builds, taskNumberCache, and commentIdCache are
	// safe for concurrent use.
	// repos maps repo URL to GitInfo. The map is read-only after NewBTCache
	// returns.
	repos     map[string]*gitinfo.GitInfo
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
	// cachedTaskComments contains BuildComments generated from the latest
	// TaskComments.
	// map[TaskComment.Repo][TaskComment.Revision][TaskComment.Name][]BuildComment
	cachedTaskComments map[string]map[string]map[string][]*buildbot.BuildComment
	// cachedTaskSpecComments contains BuilderComments generated from the latest
	// TaskSpecComments. map[BuilderName][]BuilderComment
	cachedTaskSpecComments map[string][]*buildbot.BuilderComment
}

// NewBTCache creates a combined Build and Task cache for the given repos,
// pulling data from the given buildDb and taskDb. repos maps repo URL to
// GitInfo for that repo.
func NewBTCache(repos map[string]*gitinfo.GitInfo, buildDb buildbot.DB, taskDb db.RemoteDB) (*BTCache, error) {
	tasks, err := db.NewTaskCache(taskDb, build_cache.BUILD_LOADING_PERIOD)
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
	return name + TASK_BUILDER_SUFFIX
}

// builderNameToTaskName returns the TaskSpec name if the given Builder name was
// generated from a TaskSpec name. Otherwise returns name. The second return
// value is true if a TaskSpec name, false if a Builder name.
func builderNameToTaskName(name string) (string, bool) {
	if strings.HasSuffix(name, TASK_BUILDER_SUFFIX) {
		return name[:len(name)-len(TASK_BUILDER_SUFFIX)], true
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
	if err := c.tasks.Update(); err != nil {
		return err
	}
	repos := make([]string, 0, len(c.repos))
	for k, _ := range c.repos {
		repos = append(repos, k)
	}
	comments, err := c.commentDb.GetCommentsForRepos(repos, time.Now().Add(-build_cache.BUILD_LOADING_PERIOD))
	if err != nil {
		return err
	}
	taskComments := make(map[string]map[string]map[string][]*buildbot.BuildComment, len(c.repos))
	taskSpecComments := map[string][]*buildbot.BuilderComment{}
	for _, rc := range comments {
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
	c.cachedTaskComments = taskComments
	c.cachedTaskSpecComments = taskSpecComments
	return nil
}

// findRepoForCommit returns the repo URL for the given commit, or returns an
// error if not found.
func (c *BTCache) findRepoForCommit(commit string) (string, error) {
	for name, info := range c.repos {
		if !util.TimeIsZero(info.Timestamp(commit)) {
			return name, nil
		}
	}
	return "", fmt.Errorf("Invalid commit %q", commit)
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

// taskToBuild generates a Build representing a Task.
func (c *BTCache) taskToBuild(task *db.Task) *buildbot.Build {
	results := buildbot.BUILDBOT_EXCEPTION
	switch task.Status {
	case db.TASK_STATUS_PENDING, db.TASK_STATUS_RUNNING, db.TASK_STATUS_SUCCESS:
		results = buildbot.BUILDBOT_SUCCESS
	case db.TASK_STATUS_FAILURE:
		results = buildbot.BUILDBOT_FAILURE
	case db.TASK_STATUS_MISHAP:
		results = buildbot.BUILDBOT_EXCEPTION
	}

	buildSlave := TASK_BUILD_SLAVE

	const kTasklistUrlFmt string = "https://chromium-swarm.appspot.com/newui/tasklist?columns=name&columns=state&columns=created_ts&columns=pool&filters=pool%%3ASkia&filters=%s%%3A%s&limit=20&sort=created_ts%%3Adesc"
	properties := [][]interface{}{
		{"taskURL", fmt.Sprintf("https://luci-milo.appspot.com/swarming/task/%s", task.SwarmingTaskId), PROPERTY_SOURCE},
		{"taskRetryURL", fmt.Sprintf("https://task-scheduler.skia.org/trigger?submit=true&task_spec=%s&commit=%s", task.Name, task.Revision), PROPERTY_SOURCE},
		{"taskSpecTasklistURL", fmt.Sprintf(kTasklistUrlFmt, db.SWARMING_TAG_NAME, task.Name), PROPERTY_SOURCE},
	}
	if task.SwarmingBotId != "" {
		buildSlave = task.SwarmingBotId
		properties = append(properties, [][]interface{}{
			{"botTasklistURL", fmt.Sprintf(kTasklistUrlFmt, "slavename", task.SwarmingBotId), PROPERTY_SOURCE},
			{"botDetailURL", fmt.Sprintf("https://chromium-swarm.appspot.com/restricted/bot/%s", task.SwarmingBotId), PROPERTY_SOURCE},
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

	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return &buildbot.Build{
		Builder:       taskNameToBuilderName(task.Name),
		Master:        TASK_MASTER,
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
func (c *BTCache) GetBuildsForCommits(commits []string) (map[string]map[string]*buildbot.BuildSummary, error) {
	if len(commits) == 0 {
		return map[string]map[string]*buildbot.BuildSummary{}, nil
	}

	buildResult, err := c.builds.GetBuildsForCommits(commits)
	if err != nil {
		return nil, err
	}

	// (Assume all commits are for the same repo.)
	repoName, err := c.findRepoForCommit(commits[0])
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
			buildMap[builder] = c.taskToBuild(task).GetSummary()
		}
	}
	return buildResult, nil
}

// GetBuildsForCommit returns the build data and task data (as Builds) for the
// given commit. See also BuildCache.GetBuildsForCommit.
func (c *BTCache) GetBuildsForCommit(hash string) ([]*buildbot.BuildSummary, error) {
	builds, err := c.GetBuildsForCommits([]string{hash})
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
func (c *BTCache) GetBuildsFromDateRange(from, to time.Time) ([]*buildbot.Build, error) {
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
			buildResult = append(buildResult, c.taskToBuild(task))
		}
	}
	return buildResult, nil
}

// GetBuildersComments returns comments for all builders and TaskSpecs (as
// BuilderComment). See also BuildCache.GetBuildersComments.
func (c *BTCache) GetBuildersComments() map[string][]*buildbot.BuilderComment {
	buildResult := c.builds.GetBuildersComments()
	c.mutex.RLock()
	defer c.mutex.RUnlock()
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
		return c.update()
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
		return c.update()
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
		return c.update()
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
		return c.update()
	} else {
		return c.builds.DeleteBuildComment(master, builder, number, commentId)
	}
}
