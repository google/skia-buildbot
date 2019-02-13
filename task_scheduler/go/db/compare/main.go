package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"os"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	TIME_CHUNK = 24 * time.Hour
)

var (
	local      = flag.Bool("local", false, "True if running locally.")
	boltDB     = flag.String("bolt_db", "", "Bolt DB to compare (old).")
	fsInstance = flag.String("firestore_instance", "", "Firestore instance to compare (new).")
	repoUrls   = common.NewMultiStringFlag("repo", nil, "Repositories for which to compare comments.")
	output     = flag.String("output", "compare.json", "JSON output file to write diffs.")

	BEGINNING_OF_TIME = time.Date(2016, time.September, 1, 0, 0, 0, 0, time.UTC)
	NOW               = time.Now()
)

type timeChunk struct {
	start time.Time
	end   time.Time
}

type taskDiff struct {
	Old *types.Task `json:"old,omitempty"`
	New *types.Task `json:"new,omitempty"`
}

func compareTasks(oldDB, newDB db.TaskReader, chunk *timeChunk) ([]taskDiff, error) {
	rv := []taskDiff{}
	sklog.Infof("Comparing tasks in %s - %s", chunk.start, chunk.end)
	oldTasks, err := oldDB.GetTasksFromDateRange(chunk.start, chunk.end, "")
	if err != nil {
		return nil, err
	}
	newTasks, err := newDB.GetTasksFromDateRange(chunk.start, chunk.end, "")
	if err != nil {
		return nil, err
	}
	for len(oldTasks) > 0 && len(newTasks) > 0 {
		oldTask := oldTasks[0]
		oldTask.DbModified = time.Time{}
		newTask := newTasks[0]
		newTask.DbModified = time.Time{}
		if oldTask.Created.Before(newTask.Created) {
			rv = append(rv, taskDiff{
				Old: oldTask,
			})
			oldTasks = oldTasks[1:]
		} else if newTask.Created.Before(oldTask.Created) {
			rv = append(rv, taskDiff{
				New: newTask,
			})
			newTasks = newTasks[1:]
		} else {
			if !deepequal.DeepEqual(oldTask, newTask) {
				rv = append(rv, taskDiff{
					Old: oldTask,
					New: newTask,
				})
			}
			oldTasks = oldTasks[1:]
			newTasks = newTasks[1:]
		}
	}
	for _, oldTask := range oldTasks {
		oldTask.DbModified = time.Time{}
		rv = append(rv, taskDiff{
			Old: oldTask,
		})
	}
	for _, newTask := range newTasks {
		newTask.DbModified = time.Time{}
		rv = append(rv, taskDiff{
			New: newTask,
		})
	}
	return rv, nil
}

type jobDiff struct {
	Old *types.Job `json:"old,omitempty"`
	New *types.Job `json:"new,omitempty"`
}

func compareJobs(oldDB, newDB db.JobReader, chunk *timeChunk) ([]jobDiff, error) {
	rv := []jobDiff{}
	sklog.Infof("Comparing jobs in %s - %s", chunk.start, chunk.end)
	oldJobs, err := oldDB.GetJobsFromDateRange(chunk.start, chunk.end)
	if err != nil {
		return nil, err
	}
	newJobs, err := newDB.GetJobsFromDateRange(chunk.start, chunk.end)
	if err != nil {
		return nil, err
	}
	for len(oldJobs) > 0 && len(newJobs) > 0 {
		oldJob := oldJobs[0]
		oldJob.DbModified = time.Time{}
		newJob := newJobs[0]
		newJob.DbModified = time.Time{}
		if oldJob.Created.Before(newJob.Created) {
			rv = append(rv, jobDiff{
				Old: oldJob,
			})
			oldJobs = oldJobs[1:]
		} else if newJob.Created.Before(oldJob.Created) {
			rv = append(rv, jobDiff{
				New: newJob,
			})
			newJobs = newJobs[1:]
		} else {
			if !deepequal.DeepEqual(oldJob, newJob) {
				rv = append(rv, jobDiff{
					Old: oldJob,
					New: newJob,
				})
			}
			oldJobs = oldJobs[1:]
			newJobs = newJobs[1:]
		}
	}
	for _, oldJob := range oldJobs {
		oldJob.DbModified = time.Time{}
		rv = append(rv, jobDiff{
			Old: oldJob,
		})
	}
	for _, newJob := range newJobs {
		newJob.DbModified = time.Time{}
		rv = append(rv, jobDiff{
			New: newJob,
		})
	}
	return rv, nil
}

type chunkDiff struct {
	StartTime string     `json:"startTime"`
	EndTime   string     `json:"endTime"`
	TaskDiffs []taskDiff `json:"taskDiffs,omitempty"`
	JobDiffs  []jobDiff  `json:"jobDiffs,omitempty"`
}

func compareByChunk(oldDB, newDB db.DB, output func(interface{}) error) error {
	chunks := make([]*timeChunk, 0, 1000)
	if err := util.IterTimeChunks(BEGINNING_OF_TIME, NOW, TIME_CHUNK, func(start, end time.Time) error {
		chunks = append(chunks, &timeChunk{
			start: start,
			end:   end,
		})
		return nil
	}); err != nil {
		return err
	}
	// Reverse.
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}

	for _, chunk := range chunks {
		taskDiffs, err := compareTasks(oldDB, newDB, chunk)
		if err != nil {
			return err
		}
		jobDiffs, err := compareJobs(oldDB, newDB, chunk)
		if err != nil {
			return err
		}
		if len(taskDiffs) > 0 || len(jobDiffs) > 0 {
			if err := output(chunkDiff{
				StartTime: chunk.start.String(),
				EndTime:   chunk.end.String(),
				TaskDiffs: taskDiffs,
				JobDiffs:  jobDiffs,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

type commentsDiff struct {
	Old *types.RepoComments `json:"old,omitempty"`
	New *types.RepoComments `json:"new,omitempty"`
}

func compareComments(oldDB, newDB db.CommentDB, output func(interface{}) error) error {
	oldCommentsSlice, err := oldDB.GetCommentsForRepos(*repoUrls, BEGINNING_OF_TIME)
	if err != nil {
		return err
	}
	oldCommentsMap := map[string]*types.RepoComments{}
	for _, repoComments := range oldCommentsSlice {
		oldCommentsMap[repoComments.Repo] = repoComments
	}
	newCommentsSlice, err := newDB.GetCommentsForRepos(*repoUrls, BEGINNING_OF_TIME)
	if err != nil {
		return err
	}

	for _, newRepoComments := range newCommentsSlice {
		oldRepoComments, ok := oldCommentsMap[newRepoComments.Repo]
		if !ok {
			continue
		}
		for commit, newComments := range newRepoComments.CommitComments {
			if oldComments, ok := oldRepoComments.CommitComments[commit]; ok {
				filteredNewComments := []*types.CommitComment{}
			Outer1:
				for _, newComment := range newComments {
					for oldI, oldComment := range oldComments {
						if deepequal.DeepEqual(oldComment, newComment) {
							oldComments = append(oldComments[:oldI], oldComments[oldI+1:]...)
							break Outer1
						}
					}
					filteredNewComments = append(filteredNewComments, newComment)
				}
				if len(filteredNewComments) == 0 {
					delete(newRepoComments.CommitComments, commit)
				} else {
					newRepoComments.CommitComments[commit] = filteredNewComments
				}
				if len(oldComments) == 0 {
					delete(oldRepoComments.CommitComments, commit)
				} else {
					oldRepoComments.CommitComments[commit] = oldComments
				}
			}
		}
		for commit, newTaskCommentsByName := range newRepoComments.TaskComments {
			for name, newComments := range newTaskCommentsByName {
				if oldComments, ok := oldRepoComments.TaskComments[commit][name]; ok {
					filteredNewComments := []*types.TaskComment{}
				Outer2:
					for _, newComment := range newComments {
						for oldI, oldComment := range oldComments {
							if deepequal.DeepEqual(oldComment, newComment) {
								oldComments = append(oldComments[:oldI], oldComments[oldI+1:]...)
								break Outer2
							}
						}
						filteredNewComments = append(filteredNewComments, newComment)
					}
					if len(filteredNewComments) == 0 {
						delete(newRepoComments.TaskComments[commit], name)
					} else {
						newRepoComments.TaskComments[commit][name] = filteredNewComments
					}
					if len(oldComments) == 0 {
						delete(oldRepoComments.TaskComments[commit], name)
					} else {
						oldRepoComments.TaskComments[commit][name] = oldComments
					}
				}
			}
			if len(newRepoComments.TaskComments[commit]) == 0 {
				delete(newRepoComments.TaskComments, commit)
			}
			if len(oldRepoComments.TaskComments[commit]) == 0 {
				delete(oldRepoComments.TaskComments, commit)
			}
		}
		for name, newComments := range newRepoComments.TaskSpecComments {
			if oldComments, ok := oldRepoComments.TaskSpecComments[name]; ok {
				filteredNewComments := []*types.TaskSpecComment{}
			Outer3:
				for _, newComment := range newComments {
					for oldI, oldComment := range oldComments {
						if deepequal.DeepEqual(oldComment, newComment) {
							oldComments = append(oldComments[:oldI], oldComments[oldI+1:]...)
							break Outer3
						}
					}
					filteredNewComments = append(filteredNewComments, newComment)
				}
				if len(filteredNewComments) == 0 {
					delete(newRepoComments.TaskSpecComments, name)
				} else {
					newRepoComments.TaskSpecComments[name] = filteredNewComments
				}
				if len(oldComments) == 0 {
					delete(oldRepoComments.TaskSpecComments, name)
				} else {
					oldRepoComments.TaskSpecComments[name] = oldComments
				}

			}
		}
	}

	empty := func(rc *types.RepoComments) bool {
		return len(rc.TaskComments) == 0 &&
			len(rc.TaskSpecComments) == 0 &&
			len(rc.CommitComments) == 0
	}
	for _, newRepoComments := range newCommentsSlice {
		diff := commentsDiff{}
		if !empty(newRepoComments) {
			diff.New = newRepoComments
		}
		if oldRepoComments, ok := oldCommentsMap[newRepoComments.Repo]; ok {
			if !empty(oldRepoComments) {
				diff.Old = oldRepoComments
			}
			delete(oldCommentsMap, newRepoComments.Repo)
		}
		if diff.Old != nil || diff.New != nil {
			if err := output(diff); err != nil {
				return err
			}
		}
	}
	for _, oldRepoComments := range oldCommentsMap {
		if !empty(oldRepoComments) {
			if err := output(commentsDiff{
				Old: oldRepoComments,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	common.Init()

	if *boltDB == "" {
		sklog.Fatal("--bolt_db is required.")
	}
	if *fsInstance == "" {
		sklog.Fatal("--firestore_instance is required.")
	}
	if *repoUrls == nil {
		*repoUrls = common.PUBLIC_REPOS
	}

	ctx := context.Background()
	oldDB, err := local_db.NewDB(local_db.DB_NAME, *boltDB, nil)
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(oldDB)

	ts, err := auth.NewDefaultTokenSource(*local)
	if err != nil {
		sklog.Fatal(err)
	}
	newDB, err := firestore.NewDB(ctx, firestore.FIRESTORE_PROJECT, *fsInstance, ts, nil)
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(newDB)

	f, err := os.Create(*output)
	if err != nil {
		sklog.Fatal(err)
	}
	bufW := bufio.NewWriter(f)
	enc := json.NewEncoder(bufW)
	enc.SetIndent("", "  ")
	writeObj := func(d interface{}) error {
		if err := enc.Encode(d); err != nil {
			return err
		}
		if err := bufW.Flush(); err != nil {
			return err
		}
		return f.Sync()
	}
	if err := compareComments(oldDB, newDB, writeObj); err != nil {
		sklog.Fatal(err)
	}
	if err := compareByChunk(oldDB, newDB, writeObj); err != nil {
		sklog.Fatal(err)
	}

}
