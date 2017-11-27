package dataproc

/*
   Utilities for submitting PySpark jobs to a GCE cluster.
*/

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/exec"
)

const (
	CLUSTER_SKIA  = "cluster-173d"
	JOB_ID_PREFIX = "  jobId: "
)

// PySparkJob describes a PySpark job which runs on a GCE cluster.
type PySparkJob struct {
	// The main Python file to run.
	PyFile string
	// Any additional arguments to pass onto the job command line.
	Args []string
	// Which cluster should run the job.
	Cluster string
	// Any files to provide to the job.
	Files []string

	// ID of the job, as returned by Submit().
	id string
}

// Return the command used to trigger this job.
func (j *PySparkJob) Command() *exec.Command {
	cmd := &exec.Command{
		Name: "gcloud",
		Args: []string{"dataproc", "jobs", "submit", "pyspark", "--async", "--cluster", j.Cluster, j.PyFile},
	}
	if len(j.Files) > 0 {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--files=%s", strings.Join(j.Files, ",")))
	}
	if len(j.Args) > 0 {
		cmd.Args = append(cmd.Args, "--")
		cmd.Args = append(cmd.Args, j.Args...)
	}
	return cmd
}

// Trigger the job and return its ID.
func (j *PySparkJob) Submit(ctx context.Context) (string, error) {
	if j.Cluster == "" {
		return "", fmt.Errorf("Cluster is required.")
	}
	res, err := exec.RunCommand(ctx, j.Command())
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(res, "\n") {
		if strings.HasPrefix(line, JOB_ID_PREFIX) {
			j.id = line[len(JOB_ID_PREFIX):]
			return j.id, nil
		}
	}
	return "", fmt.Errorf("Could not parse job ID from output:\n%s", res)
}

// Wait for the job to complete and return its output.
func (j *PySparkJob) Wait(ctx context.Context) (string, error) {
	return exec.RunCommand(ctx, &exec.Command{
		Name: "gcloud",
		Args: []string{"dataproc", "jobs", "wait", j.id},
	})
}

// Run the job and return its output.
func (j *PySparkJob) Run(ctx context.Context) (string, error) {
	if _, err := j.Submit(ctx); err != nil {
		return "", err
	}
	return j.Wait(ctx)
}
