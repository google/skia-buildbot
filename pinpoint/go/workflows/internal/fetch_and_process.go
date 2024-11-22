package internal

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	swarming_proto "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/read_values"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/api/iterator"
)

// CollectAndUploadParams contains required paramers for the CollectAndUploadWorkflow.
// This is the minimum set of parameters to collect results of the Swarming task runs,
// process them and upload to BigQuery.
type CollectAndUploadParams struct {
	// Project is the Google Cloud Project (ie/ chromeperf), used for creating the
	// fully qualified BQ table name.
	Project string
	// Dataset is the BQ Dataset. Format is {project}.{dataset}.
	// Used for creating the fully qualified BQ table name.
	Dataset string
	// TableName is the name of the table within Dataset. Format is
	// {project}.{dataset}.{tableName}. Used for creating the fully qualified BQ
	// table name.
	TableName string
	// Benchmark is the benchmark name, ie/ Speedometer3.
	Benchmark string
	// WorkflowID is the ID of the Temporal Workflow.
	WorkflowId string
}

// FetchAllSwarmingTasksActivity retrieves all Swarming task IDs for the workflow ID.
// It's expected that RunTestAndExportWorkflow has been run prior, where a Temporal
// workflow has triggered Swarming task IDs and has uploaded this information to BQ.
func FetchAllSwarmingTasksActivity(ctx context.Context, project, dataset, tableName, workflowId string) ([]*TestResult, error) {
	bqClient, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create BigQuery client")
	}

	fullTableName := fmt.Sprintf("%s.%s.%s", project, dataset, tableName)

	query := bqClient.Query(`
		SELECT *
		FROM ` + fullTableName + `
		WHERE
			workflow_id = @workflowId
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "workflowId", Value: workflowId},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to read from BQ")
	}

	res := []*TestResult{}
	for {
		var tr TestResult
		err := it.Next(&tr)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to parse results from BQ")
		}
		res = append(res, &tr)
	}

	return res, nil
}

// GetAllSampleValuesActivity goes through all Swarming tasks, finds the test result artifacts, and reads the values.
// The sample values are broken down by chart, so for one Swarming task (one benchmark execution), there will be
// multiple TestResult objects.
func GetAllSampleValuesActivity(ctx context.Context, benchmark string, task *TestResult) ([]*TestResult, error) {
	client, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create swarming client")
	}

	allValues := []*TestResult{}
	status, err := client.GetStatus(ctx, task.SwarmingTaskID)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to check status of job")
	}

	// if not successful, move on.
	if string(status) != swarming.TASK_STATE_COMPLETED {
		task.TaskFailed = true
		return []*TestResult{task}, nil
	}

	casRef, err := client.GetCASOutput(ctx, task.SwarmingTaskID)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to fetch CAS ref")
	}

	casClient, err := read_values.DialRBECAS(ctx, casRef.CasInstance)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create CAS client")
	}

	digests := []*swarming_proto.CASReference{casRef}
	valuesByChart, err := casClient.ReadValuesForAllCharts(ctx, benchmark, digests, "")

	for chart, sampleValues := range valuesByChart {
		newTask := &TestResult{
			WorkflowID:     task.WorkflowID,
			Bot:            task.Bot,
			SwarmingTaskID: task.SwarmingTaskID,
			Benchmark:      task.Benchmark,
			Chart:          chart,
			SampleValues:   sampleValues,
			TaskFailed:     false,
			PGOEnabled:     task.PGOEnabled,
			CreateTime:     task.CreateTime,
		}
		allValues = append(allValues, newTask)
	}

	return allValues, nil
}

// CollectAndUploadWorkflow coordinates fetching the test result artifacts from all
// Swarming tasks triggered by the RunTestAndExportWorkflow, gathering results from
// those executions by chart, and uplodaing those sample values to BQ.
func CollectAndUploadWorkflow(ctx workflow.Context, req *CollectAndUploadParams) error {
	ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
	ctx = workflow.WithActivityOptions(ctx, regularActivityOptions)
	ctx = workflow.WithLocalActivityOptions(ctx, localActivityOptions)

	firstWorkflow := req.WorkflowId
	var swarmingTasks []*TestResult
	if err := workflow.ExecuteActivity(ctx, FetchAllSwarmingTasksActivity, req.Project, req.Dataset, req.TableName, firstWorkflow).Get(ctx, &swarmingTasks); err != nil {
		return skerr.Wrap(err)
	}

	firstTask := swarmingTasks[0]

	var tasksPerChart []*TestResult
	if err := workflow.ExecuteActivity(ctx, GetAllSampleValuesActivity, req.Benchmark, firstTask).Get(ctx, &tasksPerChart); err != nil {
		return skerr.Wrap(err)
	}

	var uploadError error
	if err := workflow.ExecuteActivity(ctx, UploadResultsActivity, "chromeperf", "jeffyoon", "test", tasksPerChart).Get(ctx, &uploadError); err != nil {
		return skerr.Wrap(err)
	}

	return uploadError
}
