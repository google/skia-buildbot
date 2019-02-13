package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
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
	boltDB     = flag.String("bolt_db", "", "Bolt DB to validate.")
	fsInstance = flag.String("firestore_instance", "", "Firestore instance to validate.")

	BEGINNING_OF_TIME = time.Date(2016, time.September, 1, 0, 0, 0, 0, time.UTC)
	NOW               = time.Now()
)

type timeChunk struct {
	start time.Time
	end   time.Time
}

func validateTask(task *types.Task) error {
	if task.Id == "" {
		return fmt.Errorf("Id not set.")
	}
	if util.TimeIsZero(task.DbModified) {
		return fmt.Errorf("DbModified not set. Task %s DbModified time is %s.", task.Id, task.DbModified)
	}
	if util.TimeIsZero(task.Created) {
		return fmt.Errorf("Created not set. Task %s created time is %s.", task.Id, task.Created)
	}
	if !NOW.After(task.DbModified) {
		return fmt.Errorf("Task %s modification time is in the future: %s (current time is %s).", task.Id, task.DbModified, NOW)
	}
	return task.Validate()
}

func validateTasks(d db.TaskReader, chunks []*timeChunk) (int, error) {
	invalidCount := 0
	for _, chunk := range chunks {
		sklog.Infof("Validating tasks in %s - %s", chunk.start, chunk.end)
		tasks, err := d.GetTasksFromDateRange(chunk.start, chunk.end, "")
		if err != nil {
			return 0, err
		}
		for _, t := range tasks {
			if err := validateTask(t); err != nil {
				sklog.Errorf("%s %+v", err, t)
				invalidCount++
			}
		}
	}
	return invalidCount, nil
}

func validateJob(job *types.Job) error {
	if job.Id == "" {
		return fmt.Errorf("Id not set.")
	}
	if util.TimeIsZero(job.Created) {
		return fmt.Errorf("Created not set. Job %s created time is %s.", job.Id, job.Created)
	}
	if util.TimeIsZero(job.DbModified) {
		return fmt.Errorf("DbModified not set. Job %s DbModified time is %s.", job.Id, job.DbModified)
	}
	if !NOW.After(job.DbModified) {
		return fmt.Errorf("Job %s modification time is in the future: %s (current time is %s).", job.Id, job.DbModified, NOW)
	}
	if !job.RepoState.Valid() {
		return fmt.Errorf("Job %s RepoState is invalid.", job.Id)
	}
	return nil
}

func validateJobs(d db.JobReader, chunks []*timeChunk) (int, error) {
	invalidCount := 0
	for _, chunk := range chunks {
		sklog.Infof("Validating jobs in %s - %s", chunk.start, chunk.end)
		jobs, err := d.GetJobsFromDateRange(chunk.start, chunk.end)
		if err != nil {
			return 0, err
		}
		for _, j := range jobs {
			if err := validateJob(j); err != nil {
				sklog.Errorf("%s %+v", err, j)
				invalidCount++
			}
		}
	}
	return invalidCount, nil
}

func validate(d db.DB) error {
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

	invalidTasks, err := validateTasks(d, chunks)
	if err != nil {
		return err
	}
	invalidJobs, err := validateJobs(d, chunks)
	if err != nil {
		return err
	}
	if invalidTasks != 0 || invalidJobs != 0 {
		return fmt.Errorf("Found %d invalid tasks and %d invalid jobs.", invalidTasks, invalidJobs)
	}
	return nil
}

func main() {
	common.Init()

	if *boltDB != "" && *fsInstance != "" {
		sklog.Fatal("Only one of --bolt_db or --firestore_instance may be specified.")
	}

	ctx := context.Background()
	var d db.DBCloser
	var err error
	if *boltDB != "" {
		d, err = local_db.NewDB(local_db.DB_NAME, *boltDB, nil)
	} else if *fsInstance != "" {
		ts, err := auth.NewDefaultTokenSource(*local)
		if err != nil {
			sklog.Fatal(err)
		}
		d, err = firestore.NewDB(ctx, firestore.FIRESTORE_PROJECT, *fsInstance, ts, nil)
	} else {
		sklog.Fatal("--bolt_db or --firestore_instance is required.")
	}
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(d)

	if err := validate(d); err != nil {
		sklog.Fatal(err)
	}
}
