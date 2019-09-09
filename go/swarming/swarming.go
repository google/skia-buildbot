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
)

const (
	SWARMING_SERVER          = "chromium-swarm.appspot.com"
	SWARMING_SERVER_PRIVATE  = "chrome-swarming.appspot.com"
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
	TaskID         string // Populated after the task is triggered.
}

type ShardOutputFormat struct {
	ExitCode string `json:"exit_code"`
	Output   string `json:"output"`
}

type TaskOutputFormat struct {
	Shards []ShardOutputFormat `json:"shards"`
}

func (t *SwarmingTask) Trigger(ctx context.Context, s *SwarmingClient, hardTimeout, ioTimeout time.Duration) error {
	if err := _VerifyBinaryExists(ctx, s.SwarmingPy); err != nil {
		return fmt.Errorf("Could not find swarming binary: %s", err)
	}

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
	if err := json.NewDecoder(f).Decode(&tasks); err != nil {
		return fmt.Errorf("Could not decode %s: %s", dumpJSON, err)
	}
	t.TaskID = tasks.Tasks[t.Title].TaskID

	return nil
}

func (t *SwarmingTask) Collect(ctx context.Context, s *SwarmingClient, logStdout, logStderr bool) (string, string, error) {
	if err := _VerifyBinaryExists(ctx, s.SwarmingPy); err != nil {
		return "", "", fmt.Errorf("Could not find swarming binary: %s", err)
	}
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
	err := exec.Run(ctx, &exec.Command{
		Name:      s.SwarmingPy,
		Args:      collectArgs,
		Timeout:   t.Expiration,
		LogStdout: logStdout,
		LogStderr: logStderr,
	})
	if err != nil {
		return "", "", fmt.Errorf("Swarming trigger for %s failed with: %s", t.Title, err)
	}

	outputSummaryFile := path.Join(t.OutputDir, "summary.json")
	outputSummary, err := ioutil.ReadFile(outputSummaryFile)
	if err != nil {
		return "", "", fmt.Errorf("Could not read output summary %s: %s", outputSummaryFile, err)
	}
	var summaryOutput TaskOutputFormat
	if err := json.NewDecoder(bytes.NewReader(outputSummary)).Decode(&summaryOutput); err != nil {
		return "", "", fmt.Errorf("Could not decode %s: %s", outputSummaryFile, err)
	}

	exitCode := summaryOutput.Shards[0].ExitCode
	output := summaryOutput.Shards[0].Output
	// Directory that will contain output written to ${ISOLATED_OUTDIR}.
	outputDir := path.Join(t.OutputDir, "0")
	if exitCode != "0" {
		return output, outputDir, fmt.Errorf("Non-zero exit code: %s", summaryOutput.Shards[0].ExitCode)
	}
	return output, outputDir, nil
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
	isolateClient, err := isolate.NewClientWithServiceAccount(workDir, isolateServer, serviceAccountJSON)
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
func (s *SwarmingClient) CreateIsolatedGenJSON(isolatePath, baseDir, osType, taskName string, extraVars map[string]string, blackList []string) (string, error) {
	// Verify that isolatePath is an absolute path.
	if !path.IsAbs(isolatePath) {
		return "", fmt.Errorf("isolate path %s must be an absolute path", isolatePath)
	}

	isolatedPath := path.Join(s.WorkDir, fmt.Sprintf("%s.isolated", taskName))
	isolatedGenJSONPath := path.Join(s.WorkDir, fmt.Sprintf("%s.isolated.gen.json", taskName))
	t := &isolate.Task{
		BaseDir:     baseDir,
		Blacklist:   blackList,
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
func (s *SwarmingClient) TriggerSwarmingTasks(ctx context.Context, tasksToHashes, dimensions, tags map[string]string, cipdPackages []string, priority int, expiration, hardTimeout, ioTimeout time.Duration, idempotent, addTaskNameAsTag bool, serviceAccount string) ([]*SwarmingTask, error) {
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

func _VerifyBinaryExists(ctx context.Context, binary string) error {
	err := exec.Run(ctx, &exec.Command{
		Name:      binary,
		Args:      []string{"help"},
		Timeout:   60 * time.Second,
		LogStdout: false,
		LogStderr: true,
	})
	if err != nil {
		return fmt.Errorf("Error finding the binary %s: %s", binary, err)
	}
	return nil
}
