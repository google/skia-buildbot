package main

import (
	"context"
	"flag"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
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

	BEGINNING_OF_TIME = time.Date(2019, time.February, 1, 0, 0, 0, 0, time.UTC)
	NOW               = time.Now()
)

type timeChunk struct {
	start time.Time
	end   time.Time
}

func retry(fn func() error) error {
	sleep := 10 * time.Second
	var err error
	for i := 0; i < 5; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		sklog.Error(err)
		sklog.Errorf("Retrying in %s", sleep)
		time.Sleep(sleep)
	}
	return err
}

func migrateTasks(oldDB, newDB db.DB, chunks []*timeChunk) error {
	for _, chunk := range chunks {
		sklog.Infof("Migrating tasks in %s - %s", chunk.start, chunk.end)
		if err := retry(func() error {
			tasks, err := oldDB.GetTasksFromDateRange(chunk.start, chunk.end, "")
			if err != nil {
				return err
			}
			return newDB.PutTasksInChunks(tasks)
		}); err != nil {
			return err
		}
	}
	return nil
}

func migrateJobs(oldDB, newDB db.DB, chunks []*timeChunk) error {
	for _, chunk := range chunks {
		sklog.Infof("Migrating jobs in %s - %s", chunk.start, chunk.end)
		if err := retry(func() error {
			jobs, err := oldDB.GetJobsFromDateRange(chunk.start, chunk.end)
			if err != nil {
				return err
			}
			return newDB.PutJobsInChunks(jobs)
		}); err != nil {
			return err
		}
	}
	return nil
}

func migrateComments(oldDB, newDB db.DB) error {
	comments, err := oldDB.GetCommentsForRepos(*repoUrls, BEGINNING_OF_TIME)
	if err != nil {
		return err
	}
	for _, c := range comments {
		for _, commitComments := range c.CommitComments {
			for _, cc := range commitComments {
				if err := newDB.PutCommitComment(cc); err != nil && err != db.ErrAlreadyExists {
					return err
				}
			}
		}
		for _, taskCommentsByCommit := range c.TaskComments {
			for _, taskCommentsByName := range taskCommentsByCommit {
				for _, tc := range taskCommentsByName {
					if err := newDB.PutTaskComment(tc); err != nil && err != db.ErrAlreadyExists {
						return err
					}
				}
			}
		}
		for _, taskSpecComments := range c.TaskSpecComments {
			for _, tsc := range taskSpecComments {
				if err := newDB.PutTaskSpecComment(tsc); err != nil && err != db.ErrAlreadyExists {
					return err
				}
			}
		}
	}
	return nil
}

func migrate(oldDB, newDB db.DB) error {
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
	for i, j := 0, len(chunks)-1; i < j; i, j = i+1, j-1 {
		chunks[i], chunks[j] = chunks[j], chunks[i]
	}

	if err := migrateTasks(oldDB, newDB, chunks); err != nil {
		return err
	}
	if err := migrateJobs(oldDB, newDB, chunks); err != nil {
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
	if err := migrate(oldDB, newDB); err != nil {
		sklog.Fatal(err)
	}
}
