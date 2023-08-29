package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"

	"go.skia.org/infra/cabe/go/backends"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/workflowexecutions/v1"
)

// flag names
const (
	gcpProjectNameFlagName  = "project"
	gcpLocationFlagName     = "location"
	gcpWorkflowNameFlagName = "workflow"
	attemptCountFlagName    = "attempts"
	dryRunFlagName          = "dry-run"
	executionIDFlagName     = "execution"
)

// benchmarkArguments is a struct for reading entities from pinpoint's Datastore instance.
type benchmarkArguments struct {
	Benchmark string `datastore:"benchmark"`
	Chart     string `datastore:"chart"`
	Statistic string `datastore:"statistic"`
	Story     string `datastore:"story"`
}

// pinpointJob is a struct for reading entities from pinpoint's Datastore instance.
type pinpointJob struct {
	Arguments          []byte             `datastore:"arguments"` // base64-encoded JSON
	BenchmarkArguments benchmarkArguments `datastore:"benchmark_arguments"`
	ComparisonMode     string             `datastore:"comparison_mode"`
	Configuration      string             `datastore:"configuration"`
}

// stats is a struct for reading results from cloud workflow executions.
type stats struct {
	Lower    float64 `json:lower`
	Upper    float64 `json:upper`
	PValue   float64 `json:"p_value"`
	CtrlMed  float64 `json:"control_median"`
	TreatMed float64 `json:"treatment_median"`
}

// workflowResult is a struct for reading results from cloud workflow executions.
type workflowResult struct {
	JobId    string `json:"job_id"`
	Decision bool   `json:"decision"`
	Stats    *stats `json:"statistic"`
}

// sandwichCmd holds the flag values and any internal state necessary for
// executing the `sandwich` subcommand. It re-runs a pinpoint auto-bisect
// job through a "sandwich verification" cloud workflow execution.
// This command runs in dry-run mode by default, so you need to specify
// -dry-run=false to get it to actually start the workflow or make any
// other calls to the workflows service.
type sandwichCmd struct {
	commonCmd
	gcpProjectName  string
	gcpLocation     string
	gcpWorkflowName string
	attemptCount    int
	dryRun          bool
	executionID     string
}

// SandwichCommand returns a [*cli.Command] for executing a sandwich verification workflow to verify a regression from a pinpoint bisect job.
func SandwichCommand() *cli.Command {
	cmd := &sandwichCmd{}
	return &cli.Command{
		Name:        "sandwich",
		Description: "sandwich executes a verification workflow on a bisected regression.",
		Usage:       "cabe sandwich -- --pinpoint-job <pinpoint-job>",
		Flags:       cmd.flags(),
		Action:      cmd.action,
		After:       cmd.cleanup,
	}
}

func (cmd *sandwichCmd) flags() []cli.Flag {
	fl := []cli.Flag{
		&cli.StringFlag{
			Name:        gcpProjectNameFlagName,
			Value:       "chromeperf",
			Usage:       "GAE project app name",
			Destination: &cmd.gcpProjectName,
		}, &cli.StringFlag{
			Name:        gcpLocationFlagName,
			Value:       "us-central1",
			Usage:       "location for workflow execution",
			Destination: &cmd.gcpLocation,
		}, &cli.StringFlag{
			Name:        gcpWorkflowNameFlagName,
			Value:       "sandwich-verification-workflow-prod",
			Usage:       "name of workflow to execute",
			Destination: &cmd.gcpWorkflowName,
		}, &cli.IntFlag{
			Name:        attemptCountFlagName,
			Value:       30,
			Usage:       "iterations verification job will run",
			Destination: &cmd.attemptCount,
		}, &cli.BoolFlag{
			Name:        dryRunFlagName,
			Value:       true,
			Usage:       "dry run for StartWorkflow; just print CreateExecutionRequest to stdout",
			Destination: &cmd.dryRun,
		}, &cli.StringFlag{
			Name:        executionIDFlagName,
			Value:       "",
			Usage:       "execution id of the workflow",
			Destination: &cmd.executionID,
		},
	}
	return append(fl, cmd.commonCmd.flags()...)
}

func (cmd *sandwichCmd) datastoreClient(ctx context.Context) (dsClient *datastore.Client, err error) {
	dsClient, err = datastore.NewClient(ctx, cmd.gcpProjectName)
	return
}

func (cmd *sandwichCmd) workflowExecutionsClient(ctx context.Context) (wfes *workflowexecutions.ProjectsLocationsWorkflowsExecutionsService, err error) {
	if !cmd.dryRun {
		s, err := backends.DialCloudWorkflowsExecutionService(ctx)
		if err == nil {
			wfes = s.Projects.Locations.Workflows.Executions
		}
	}
	return
}

func (cmd *sandwichCmd) action(cliCtx *cli.Context) error {
	ctx := cliCtx.Context

	if cmd.commonCmd.pinpointJobID != "" {
		return cmd.startWorkflow(ctx)
	} else if cmd.executionID != "" {
		return cmd.checkWorkflow(ctx)
	} else {
		return fmt.Errorf("please provide a Pinpoint Job ID or a Workflow Execution ID")
	}
	return nil
}

func (cmd *sandwichCmd) startWorkflow(ctx context.Context) error {
	dsClient, err := cmd.datastoreClient(ctx)
	defer dsClient.Close()

	pinpointJobIDInt, err := strconv.ParseInt(cmd.pinpointJobID, 16, 64)
	if err != nil {
		return err
	}
	pinpointJobKey := datastore.IDKey("Job", pinpointJobIDInt, nil)
	job := &pinpointJob{}
	err = dsClient.Get(ctx, pinpointJobKey, job)
	if err != nil &&
		// struct PinpointJob is a subset of fields that are in datastore, so ignore errors about missing fields on it.
		!strings.Contains(err.Error(), "no such struct field") {
		return err
	}
	if job.ComparisonMode != "performance" {
		return fmt.Errorf("%s is not an autobisect job (its comparison_mode is '%s')\n",
			cmd.pinpointJobID, job.ComparisonMode)
	}
	arguments := map[string]interface{}{}
	err = json.Unmarshal(job.Arguments, &arguments)
	if err != nil {
		return err
	}

	benchmark := arguments["benchmark"]
	botName := arguments["configuration"]
	story := arguments["story"]
	target := arguments["target"]
	measurement := arguments["chart"]
	startGitHash := arguments["start_git_hash"]
	endGitHash := arguments["end_git_hash"]

	wfes, err := cmd.workflowExecutionsClient(ctx)
	if err != nil {
		return err
	}

	workflowArgs := map[string]interface{}{
		"anomaly": map[string]interface{}{
			"benchmark":      benchmark,
			"bot_name":       botName,
			"story":          story,
			"measurement":    measurement,
			"target":         target,
			"start_git_hash": startGitHash,
			"end_git_hash":   endGitHash,
			"attempt_count":  cmd.attemptCount,
		},
	}
	encodedArgs, err := json.Marshal(workflowArgs)
	if err != nil {
		return err
	}
	parentName := fmt.Sprintf("projects/%s/locations/%s/workflows/%s", cmd.gcpProjectName, cmd.gcpLocation, cmd.gcpWorkflowName)
	requestedExecution := &workflowexecutions.Execution{
		Argument:     string(encodedArgs),
		CallLogLevel: "LOG_ALL_CALLS",
	}
	if cmd.dryRun {
		fmt.Printf("createCall: %+v\n", requestedExecution)
		return nil
	}
	createCall := wfes.Create(parentName, requestedExecution)
	execution, err := createCall.Do()
	if err != nil {
		return err
	}
	fmt.Printf("Workflow execution created. Check status with\n\ngo run sandwich.go -execution-id %s\n\n", execution.Name)
	return nil
}

func (cmd *sandwichCmd) checkWorkflow(ctx context.Context) error {
	wfes, err := cmd.workflowExecutionsClient(ctx)
	if err != nil {
		return err
	}
	executionName := fmt.Sprintf("projects/%s/locations/%s/workflows/%s/executions/%s",
		cmd.gcpProjectName, cmd.gcpLocation, cmd.gcpWorkflowName, cmd.executionID)
	if cmd.dryRun {
		fmt.Printf("getCall: %+v\n", executionName)
		return nil
	}
	getCall := wfes.Get(executionName)
	execution, err := getCall.Do()
	if err != nil {
		return err
	}

	executionStr, err := json.MarshalIndent(execution, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("execution:\n%s\n", executionStr)
	if len(execution.Result) != 0 {
		result := &workflowResult{}
		err = json.Unmarshal([]byte(execution.Result), &result)
		if err != nil {
			return nil
		}
		fmt.Printf("WorkflowResult: exec_id: %v verification job: %v decision: %v lower: %v upper: %v p-value %v ctrl_med %v treat_med %v \n",
			cmd.executionID,
			result.JobId,
			result.Decision,
			result.Stats.Lower,
			result.Stats.Upper,
			result.Stats.PValue,
			result.Stats.CtrlMed,
			result.Stats.TreatMed,
		)
	}
	return nil
}
