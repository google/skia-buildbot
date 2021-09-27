package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/firestore"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	TIME_CHUNK = 24 * time.Hour
)

var (
	local      = flag.Bool("local", false, "True if running locally.")
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
	if !time.Now().After(task.DbModified) {
		return fmt.Errorf("Task %s modification time is in the future: %s (current time is %s).", task.Id, task.DbModified, NOW)
	}
	return task.Validate()
}

func validateTasks(ctx context.Context, d db.TaskReader, chunks []*timeChunk) ([]string, error) {
	invalidIds := []string{}
	for _, chunk := range chunks {
		sklog.Infof("Validating tasks in %s - %s", chunk.start, chunk.end)
		tasks, err := d.GetTasksFromDateRange(ctx, chunk.start, chunk.end, "")
		if err != nil {
			return nil, err
		}
		for _, t := range tasks {
			if err := validateTask(t); err != nil {
				sklog.Errorf("%s %+v", err, t)
				invalidIds = append(invalidIds, t.Id)
				continue
			}
		}
	}
	return invalidIds, nil
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
	if !time.Now().After(job.DbModified) {
		return fmt.Errorf("Job %s modification time is in the future: %s (current time is %s).", job.Id, job.DbModified, NOW)
	}
	if !job.RepoState.Valid() {
		return fmt.Errorf("Job %s RepoState is invalid.", job.Id)
	}
	return nil
}

func validateJobs(ctx context.Context, d db.JobReader, chunks []*timeChunk) ([]string, error) {
	invalidIds := []string{}
	for _, chunk := range chunks {
		sklog.Infof("Validating jobs in %s - %s", chunk.start, chunk.end)
		jobs, err := d.GetJobsFromDateRange(ctx, chunk.start, chunk.end, "")
		if err != nil {
			return nil, err
		}
		for _, j := range jobs {
			if err := validateJob(j); err != nil {
				sklog.Errorf("%s %+v", err, j)
				invalidIds = append(invalidIds, j.Id)
			}
		}
	}
	return invalidIds, nil
}

func validate(ctx context.Context, d db.DB) error {
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

	invalidTasks, err := validateTasks(ctx, d, chunks)
	if err != nil {
		return err
	}
	invalidJobs, err := validateJobs(ctx, d, chunks)
	if err != nil {
		return err
	}
	if len(invalidTasks) != 0 || len(invalidJobs) != 0 {
		return fmt.Errorf("Found %d invalid tasks and %d invalid jobs.\nTasks:\n%s\nJobs:\n%s", len(invalidTasks), len(invalidJobs), strings.Join(invalidTasks, "\n"), strings.Join(invalidJobs, "\n"))
	}
	sklog.Infof("All tasks and jobs are valid!")
	return nil
}

func main() {
	common.Init()

	if *fsInstance == "" {
		sklog.Fatal("--firestore_instance is required.")
	}

	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(*local)
	if err != nil {
		sklog.Fatal(err)
	}
	d, err := firestore.NewDBWithParams(ctx, firestore.FIRESTORE_PROJECT, *fsInstance, ts)
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(d)

	if err := validate(ctx, d); err != nil {
		sklog.Fatal(err)
	}
}
