/*
	Utilities for isolating and swarming. See swarming_test.go for usage examples.
*/
package swarming

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/isolate"
)

const (
	SWARMING_SERVER          = "chromium-swarm.appspot.com"
	LUCI_CLIENT_REPO         = "https://github.com/luci/client-py"
	RECOMMENDED_IO_TIMEOUT   = 20 * time.Minute
	RECOMMENDED_HARD_TIMEOUT = 1 * time.Hour
	RECOMMENDED_PRIORITY     = 90
	RECOMMENDED_EXPIRATION   = 4 * time.Hour
)

type SwarmingClient struct {
	WorkDir        string
	isolateClient  *isolate.Client
	SwarmingServer string
}

type SwarmingTask struct {
	Title        string
	IsolatedHash string
	OutputDir    string
	Dimensions   map[string]string
	Tags         map[string]string
	Priority     int
	Expiration   time.Duration
	Idempotent   bool
}

type ShardOutputFormat struct {
	ExitCode string `json:"exit_code"`
	Output   string `json:"output"`
}

type TaskOutputFormat struct {
	Shards []ShardOutputFormat `json:"shards"`
}

func (t *SwarmingTask) Trigger(s *SwarmingClient, hardTimeout, ioTimeout time.Duration) error {
	swarmingBinary := path.Join(s.WorkDir, "client-py", "swarming.py")
	if err := _VerifyBinaryExists(swarmingBinary); err != nil {
		return fmt.Errorf("Could not find swarming binary: %s", err)
	}

	// Run swarming trigger.
	dumpJSON := path.Join(t.OutputDir, fmt.Sprintf("%s-trigger-output.json", t.Title))
	triggerArgs := []string{
		"trigger",
		"--swarming", s.SwarmingServer,
		"--isolate-server", s.isolateClient.ServerUrl,
		"--priority", strconv.Itoa(t.Priority),
		"--shards", strconv.Itoa(1),
		"--task-name", t.Title,
		"--dump-json", dumpJSON,
		"--expiration", strconv.FormatFloat(t.Expiration.Seconds(), 'f', 0, 64),
		"--io-timeout", strconv.FormatFloat(ioTimeout.Seconds(), 'f', 0, 64),
		"--hard-timeout", strconv.FormatFloat(hardTimeout.Seconds(), 'f', 0, 64),
		"--verbose",
	}
	for k, v := range t.Dimensions {
		triggerArgs = append(triggerArgs, "--dimension", k, v)
	}
	for k, v := range t.Tags {
		triggerArgs = append(triggerArgs, "--tag", fmt.Sprintf("%s:%s", k, v))
	}
	if t.Idempotent {
		triggerArgs = append(triggerArgs, "--idempotent")
	}
	triggerArgs = append(triggerArgs, t.IsolatedHash)

	err := exec.Run(&exec.Command{
		Name: swarmingBinary,
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
	return nil
}

func (t *SwarmingTask) Collect(s *SwarmingClient) (string, string, error) {
	swarmingBinary := path.Join(s.WorkDir, "client-py", "swarming.py")
	if err := _VerifyBinaryExists(swarmingBinary); err != nil {
		return "", "", fmt.Errorf("Could not find swarming binary: %s", err)
	}
	dumpJSON := path.Join(t.OutputDir, fmt.Sprintf("%s-trigger-output.json", t.Title))

	// Run swarming collect.
	collectArgs := []string{
		"collect",
		"--json", dumpJSON,
		"--swarming", SWARMING_SERVER,
		"--task-output-dir", t.OutputDir,
		"--verbose",
	}
	err := exec.Run(&exec.Command{
		Name:      swarmingBinary,
		Args:      collectArgs,
		Timeout:   t.Expiration,
		LogStdout: true,
		LogStderr: true,
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
func NewSwarmingClient(workDir string) (*SwarmingClient, error) {
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		if err := os.MkdirAll(workDir, 0700); err != nil {
			return nil, fmt.Errorf("Could not create %s: %s", workDir, err)
		}
	}
	// Checkout luci client-py to get access to swarming.py for triggering and
	// collecting tasks.
	if _, err := gitinfo.CloneOrUpdate(LUCI_CLIENT_REPO, path.Join(workDir, "client-py"), false); err != nil {
		return nil, fmt.Errorf("Could not checkout %s: %s", LUCI_CLIENT_REPO, err)
	}

	// Create an isolate client.
	isolateClient, err := isolate.NewClient(workDir)
	if err != nil {
		return nil, fmt.Errorf("Failed to create isolate client: %s", err)
	}

	return &SwarmingClient{
		WorkDir:        workDir,
		isolateClient:  isolateClient,
		SwarmingServer: SWARMING_SERVER,
	}, nil
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
func (s *SwarmingClient) BatchArchiveTargets(isolatedGenJSONs []string, d time.Duration) (map[string]string, error) {
	// Run isolate batcharchive.
	dumpJSON := path.Join(s.WorkDir, "isolate-output.json")
	if err := s.isolateClient.BatchArchiveTasks(isolatedGenJSONs, dumpJSON); err != nil {
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
func (s *SwarmingClient) TriggerSwarmingTasks(tasksToHashes, dimensions, tags map[string]string, priority int, expiration, hardTimeout, ioTimeout time.Duration, idempotent bool) ([]*SwarmingTask, error) {
	tasks := []*SwarmingTask{}

	for taskName, hash := range tasksToHashes {
		taskOutputDir := path.Join(s.WorkDir, taskName)
		if err := os.MkdirAll(taskOutputDir, 0700); err != nil {
			return nil, fmt.Errorf("Could not create %s: %s", taskOutputDir, err)
		}
		task := &SwarmingTask{
			Title:        taskName,
			IsolatedHash: hash,
			OutputDir:    taskOutputDir,
			Dimensions:   dimensions,
			Tags:         tags,
			Priority:     priority,
			Expiration:   expiration,
			Idempotent:   idempotent,
		}
		if err := task.Trigger(s, hardTimeout, ioTimeout); err != nil {
			return nil, fmt.Errorf("Could not trigger task %s: %s", taskName, err)
		}
		glog.Infof("Triggered the task: %v", task)
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (s *SwarmingClient) Cleanup() {
	if err := os.RemoveAll(s.WorkDir); err != nil {
		glog.Errorf("Could not cleanup swarming work dir: %s", err)
	}
}

func _VerifyBinaryExists(binary string) error {
	err := exec.Run(&exec.Command{
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
