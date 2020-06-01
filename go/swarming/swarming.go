/*
	Utilities for isolating and swarming. See swarming_test.go for usage examples.
*/
package swarming

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	SWARMING_SERVER          = "chromium-swarm.appspot.com"
	SWARMING_SERVER_PRIVATE  = "chrome-swarming.appspot.com"
	SWARMING_SERVER_DEV      = "chromium-swarm-dev.appspot.com"
	LUCI_CLIENT_REPO         = "https://chromium.googlesource.com/infra/luci/client-py"
	RECOMMENDED_IO_TIMEOUT   = 20 * time.Minute
	RECOMMENDED_HARD_TIMEOUT = 1 * time.Hour
	RECOMMENDED_PRIORITY     = 90
	RECOMMENDED_EXPIRATION   = 4 * time.Hour
	// "priority 0 can only be used for terminate request"
	HIGHEST_PRIORITY = 1
	LOWEST_PRIORITY  = 255
)

type SwarmingClient struct {
	WorkDir            string
	isolateClient      *isolate.Client
	isolateServer      string
	SwarmingPy         string
	SwarmingServer     string
	ServiceAccountJSON string
}

type SwarmingTask struct {
	Title          string
	IsolatedHash   string
	OutputDir      string
	Dimensions     map[string]string
	Tags           map[string]string
	CipdPackages   []string
	Priority       int
	Expiration     time.Duration
	Idempotent     bool
	ServiceAccount string
	EnvPrefixes    map[string]string
	TaskID         string // Populated after the task is triggered.
}

type ShardOutputFormat struct {
	Output string `json:"output"`
	State  string `json:"state"`
}

type TaskOutputFormat struct {
	Shards []ShardOutputFormat `json:"shards"`
}

func (t *SwarmingTask) Trigger(ctx context.Context, s *SwarmingClient, hardTimeout, ioTimeout time.Duration) error {

	// Run swarming trigger.
	dumpJSON := path.Join(t.OutputDir, fmt.Sprintf("%s-trigger-output.json", t.Title))
	triggerArgs := []string{
		"trigger",
		"--swarming", s.SwarmingServer,
		"--isolate-server", s.isolateServer,
		"--priority", strconv.Itoa(t.Priority),
		"--shards", strconv.Itoa(1),
		"--task-name", t.Title,
		"--dump-json", dumpJSON,
		"--expiration", strconv.FormatFloat(t.Expiration.Seconds(), 'f', 0, 64),
		"--io-timeout", strconv.FormatFloat(ioTimeout.Seconds(), 'f', 0, 64),
		"--hard-timeout", strconv.FormatFloat(hardTimeout.Seconds(), 'f', 0, 64),
		"--verbose",
	}
	if t.ServiceAccount != "" {
		triggerArgs = append(triggerArgs, "--service-account", t.ServiceAccount)
	}
	for k, v := range t.Dimensions {
		triggerArgs = append(triggerArgs, "--dimension", k, v)
	}
	for k, v := range t.Tags {
		triggerArgs = append(triggerArgs, "--tag", fmt.Sprintf("%s:%s", k, v))
	}
	for _, c := range t.CipdPackages {
		triggerArgs = append(triggerArgs, "--cipd-package", c)
	}
	for k, v := range t.EnvPrefixes {
		triggerArgs = append(triggerArgs, "--env-prefix", k, v)
	}
	if t.Idempotent {
		triggerArgs = append(triggerArgs, "--idempotent")
	}
	if s.ServiceAccountJSON != "" {
		triggerArgs = append(triggerArgs, "--auth-service-account-json", s.ServiceAccountJSON)
	}
	triggerArgs = append(triggerArgs, "--isolated", t.IsolatedHash)

	err := exec.Run(ctx, &exec.Command{
		Name: s.SwarmingPy,
		Args: triggerArgs,
		// Triggering a task should be immediate. Setting a 15m timeout incase
		// something goes wrong.
		Timeout:   15 * time.Minute,
		LogStdout: true,
		LogStderr: true,
	})
	if err != nil {
		return fmt.Errorf("Swarming trigger for %s failed with: %s", t.Title, err)
	}

	// Read the taskID from the dumpJSON and set it to the task object.
	type Task struct {
		TaskID string `json:"task_id"`
	}
	type Tasks struct {
		Tasks map[string]Task `json:"tasks"`
	}
	var tasks Tasks
	f, err := os.Open(dumpJSON)
	if err != nil {
		return err
	}
	defer util.Close(f)
	if err := json.NewDecoder(f).Decode(&tasks); err != nil {
		return fmt.Errorf("Could not decode %s: %s", dumpJSON, err)
	}
	t.TaskID = tasks.Tasks[t.Title].TaskID

	return nil
}

// Collect collects the swarming task. It is a blocking call that returns only after the task
// completes. It returns the following:
// * Output of the task.
// * Location of the ${ISOLATED_OUTDIR}.
// * State of the task. Eg: COMPLETED/KILLED.
// * Error is non-nil if something goes wrong. If the command to collect returns a non-zero exit
//   code then error is non-nil but all of the above (output, outdir, state) are also returned if
//   known. This is useful for checking if a task failed because it was cancelled.
func (t *SwarmingTask) Collect(ctx context.Context, s *SwarmingClient, logStdout, logStderr bool) (string, string, string, error) {
	dumpJSON := path.Join(t.OutputDir, fmt.Sprintf("%s-trigger-output.json", t.Title))

	// Run swarming collect.
	collectArgs := []string{
		"collect",
		"--json", dumpJSON,
		"--swarming", s.SwarmingServer,
		"--task-output-dir", t.OutputDir,
		"--verbose",
	}
	if s.ServiceAccountJSON != "" {
		collectArgs = append(collectArgs, "--auth-service-account-json", s.ServiceAccountJSON)
	}
	collectCmdErr := exec.Run(ctx, &exec.Command{
		Name:      s.SwarmingPy,
		Args:      collectArgs,
		Timeout:   t.Expiration,
		LogStdout: logStdout,
		LogStderr: logStderr,
	})

	// Read and parse the summary file if it exists before checking for the error.
	outputSummaryFile := path.Join(t.OutputDir, "summary.json")
	output := ""
	state := ""
	if _, statErr := os.Stat(outputSummaryFile); statErr == nil {

		outputSummary, readErr := ioutil.ReadFile(outputSummaryFile)
		if readErr != nil {
			return "", "", "", fmt.Errorf("Could not read output summary %s: %s", outputSummaryFile, readErr)
		}
		var summaryOutput TaskOutputFormat
		if decodeErr := json.NewDecoder(bytes.NewReader(outputSummary)).Decode(&summaryOutput); decodeErr != nil {
			return "", "", "", fmt.Errorf("Could not decode %s: %s", outputSummaryFile, decodeErr)
		}
		output = summaryOutput.Shards[0].Output
		state = summaryOutput.Shards[0].State
	}

	// Directory that will contain output written to ${ISOLATED_OUTDIR}.
	outputDir := path.Join(t.OutputDir, "0")
	if collectCmdErr != nil {
		return output, outputDir, state, fmt.Errorf("Swarming collect for %s failed with: %s", t.Title, collectCmdErr)
	}
	return output, outputDir, state, nil
}

// NewSwarmingClient returns an instance of Swarming populated with default
// values.
func NewSwarmingClient(ctx context.Context, workDir, swarmingServer, isolateServer, serviceAccountJSON string) (*SwarmingClient, error) {
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workDir, 0700); err != nil {
			return nil, fmt.Errorf("Could not create %s: %s", workDir, err)
		}
	}
	// Checkout luci client-py to get access to swarming.py for triggering and
	// collecting tasks.
	luciClient, err := git.NewCheckout(ctx, LUCI_CLIENT_REPO, workDir)
	if err != nil {
		return nil, fmt.Errorf("Could not checkout %s: %s", LUCI_CLIENT_REPO, err)
	}
	if err := luciClient.Update(ctx); err != nil {
		return nil, err
	}
	swarmingPy := path.Join(luciClient.Dir(), "swarming.py")

	// Create an isolate client.
	isolateClient, err := isolate.NewLegacyClientWithServiceAccount(workDir, isolateServer, serviceAccountJSON)
	if err != nil {
		return nil, fmt.Errorf("Failed to create isolate client: %s", err)
	}

	return &SwarmingClient{
		WorkDir:            workDir,
		isolateClient:      isolateClient,
		isolateServer:      isolateServer,
		SwarmingPy:         swarmingPy,
		SwarmingServer:     swarmingServer,
		ServiceAccountJSON: serviceAccountJSON,
	}, nil
}

func (s *SwarmingClient) GetIsolateClient() *isolate.Client {
	return s.isolateClient
}

// CreateIsolatedGenJSON creates isolated.gen.json files in the work dir. They then
// can be passed on to BatchArchiveTargets.
func (s *SwarmingClient) CreateIsolatedGenJSON(isolatePath, baseDir, osType, taskName string, extraVars map[string]string) (string, error) {
	// Verify that isolatePath is an absolute path.
	if !path.IsAbs(isolatePath) {
		return "", fmt.Errorf("isolate path %s must be an absolute path", isolatePath)
	}

	isolatedPath := path.Join(s.WorkDir, fmt.Sprintf("%s.isolated", taskName))
	isolatedGenJSONPath := path.Join(s.WorkDir, fmt.Sprintf("%s.isolated.gen.json", taskName))
	t := &isolate.Task{
		BaseDir:     baseDir,
		ExtraVars:   extraVars,
		IsolateFile: isolatePath,
		OsType:      osType,
	}
	if err := isolate.WriteIsolatedGenJson(t, isolatedGenJSONPath, isolatedPath); err != nil {
		return "", err
	}
	return isolatedGenJSONPath, nil
}

// BatchArchiveTargets batcharchives the specified isolated.gen.json files.
func (s *SwarmingClient) BatchArchiveTargets(ctx context.Context, isolatedGenJSONs []string, d time.Duration) (map[string]string, error) {
	// Run isolate batcharchive.
	dumpJSON := path.Join(s.WorkDir, "isolate-output.json")
	if err := s.isolateClient.BatchArchiveTasks(ctx, isolatedGenJSONs, dumpJSON); err != nil {
		return nil, err
	}

	// Read the isolate hashes from the dump JSON.
	dumpFile, err := ioutil.ReadFile(dumpJSON)
	if err != nil {
		return nil, fmt.Errorf("Could not read JSON output %s: %s", dumpJSON, err)
	}
	var tasksToHashes map[string]string
	if err := json.NewDecoder(bytes.NewReader(dumpFile)).Decode(&tasksToHashes); err != nil {
		return nil, fmt.Errorf("Could not decode %s: %s", dumpJSON, err)
	}

	return tasksToHashes, nil
}

// Trigger swarming using the specified hashes and dimensions.
func (s *SwarmingClient) TriggerSwarmingTasks(ctx context.Context, tasksToHashes, dimensions, tags, envPrefixes map[string]string, cipdPackages []string, priority int, expiration, hardTimeout, ioTimeout time.Duration, idempotent, addTaskNameAsTag bool, serviceAccount string) ([]*SwarmingTask, error) {
	tasks := []*SwarmingTask{}

	for taskName, hash := range tasksToHashes {
		taskOutputDir := path.Join(s.WorkDir, taskName)
		if err := os.MkdirAll(taskOutputDir, 0700); err != nil {
			return nil, fmt.Errorf("Could not create %s: %s", taskOutputDir, err)
		}
		taskTags := map[string]string{}
		for k, v := range tags {
			taskTags[k] = v
		}
		if addTaskNameAsTag {
			taskTags["name"] = taskName
		}
		task := &SwarmingTask{
			Title:          taskName,
			IsolatedHash:   hash,
			OutputDir:      taskOutputDir,
			Dimensions:     dimensions,
			Tags:           taskTags,
			CipdPackages:   cipdPackages,
			Priority:       priority,
			Expiration:     expiration,
			Idempotent:     idempotent,
			ServiceAccount: serviceAccount,
			EnvPrefixes:    envPrefixes,
		}
		if err := task.Trigger(ctx, s, hardTimeout, ioTimeout); err != nil {
			return nil, fmt.Errorf("Could not trigger task %s: %s", taskName, err)
		}
		sklog.Infof("Triggered the task: %v", task)
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (s *SwarmingClient) Cleanup() {
	if err := os.RemoveAll(s.WorkDir); err != nil {
		sklog.Errorf("Could not cleanup swarming work dir: %s", err)
	}
}
