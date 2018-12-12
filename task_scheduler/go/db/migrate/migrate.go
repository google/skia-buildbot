package main

import (
	"context"
	"flag"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	fs "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
)

const (
	TIME_CHUNK = 24 * time.Hour
)

var (
	local      = flag.Bool("local", false, "True if running locally.")
	boltDB     = flag.String("bolt_db", "", "Bolt DB to migrate from.")
	fsInstance = flag.String("firestore_instance", "", "Firestore instance to migrate to.")
	repoUrls   = common.NewMultiStringFlag("repo", nil, "Repositories for which to schedule tasks.")

	BEGINNING_OF_TIME = time.Date(2016, time.September, 1, 0, 0, 0, 0, time.UTC)
	NOW               = time.Now()
)

func migrateTasks(oldDB, newDB db.DB) error {
	return util.IterTimeChunks(BEGINNING_OF_TIME, NOW, TIME_CHUNK, func(start, end time.Time) error {
		sklog.Infof("Migrating tasks in %s - %s", start, end)
		tasks, err := oldDB.GetTasksFromDateRange(start, end, "")
		if err != nil {
			return err
		}
		for _, t := range tasks {
			t.DbModified = time.Time{}
		}
		return util.ChunkIter(len(tasks), fs.MAX_TRANSACTION_DOCS, func(start, end int) error {
			return newDB.PutTasks(tasks[start:end])
		})
	})
}

func migrateJobs(oldDB, newDB db.DB) error {
	return util.IterTimeChunks(BEGINNING_OF_TIME, NOW, TIME_CHUNK, func(start, end time.Time) error {
		sklog.Infof("Migrating jobs in %s - %s", start, end)
		jobs, err := oldDB.GetJobsFromDateRange(start, end)
		if err != nil {
			return err
		}
		for _, j := range jobs {
			j.DbModified = time.Time{}
		}
		return util.ChunkIter(len(jobs), fs.MAX_TRANSACTION_DOCS, func(start, end int) error {
			return newDB.PutJobs(jobs[start:end])
		})
	})
}

func migrateComments(oldDB, newDB db.DB) error {
	comments, err := oldDB.GetCommentsForRepos(*repoUrls, BEGINNING_OF_TIME)
	if err != nil {
		return err
	}
	for _, c := range comments {
		for _, commitComments := range c.CommitComments {
			for _, cc := range commitComments {
				if err := newDB.PutCommitComment(cc); err != nil {
					return err
				}
			}
		}
		for _, taskCommentsByCommit := range c.TaskComments {
			for _, taskCommentsByName := range taskCommentsByCommit {
				for _, tc := range taskCommentsByName {
					if err := newDB.PutTaskComment(tc); err != nil {
						return err
					}
				}
			}
		}
		for _, taskSpecComments := range c.TaskSpecComments {
			for _, tsc := range taskSpecComments {
				if err := newDB.PutTaskSpecComment(tsc); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func migrate(oldDB, newDB db.DB) error {
	if err := migrateTasks(oldDB, newDB); err != nil {
		return err
	}
	if err := migrateJobs(oldDB, newDB); err != nil {
		return err
	}
	if err := migrateComments(oldDB, newDB); err != nil {
		return err
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
	oldDB, err := local_db.NewDB(local_db.DB_NAME, *boltDB, nil, nil)
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(oldDB)

	ts, err := auth.NewDefaultTokenSource(*local)
	if err != nil {
		sklog.Fatal(err)
	}
	newDB, err := firestore.NewDB(ctx, firestore.FIRESTORE_PROJECT, *fsInstance, ts, nil, nil)
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(newDB)
	if err := migrate(oldDB, newDB); err != nil {
		sklog.Fatal(err)
	}
}
