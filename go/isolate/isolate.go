package isolate

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// Client is a Skia-specific wrapper around the Isolate executable.
type Client struct {
	casInstance   string
	isolate       string
	isolateserver string
	workdir       string
}

// NewClient returns a Client instance which expects to find the "isolate" and
// "isolated" binaries in PATH. Typically they should be obtained via CIPD.
func NewClient(workdir, casInstance string) (*Client, error) {
	if workdir == "" {
		return nil, skerr.Fmt("workdir is required")
	}
	if casInstance == "" {
		return nil, skerr.Fmt("casInstance is required")
	}
	absPath, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	return &Client{
		casInstance:   casInstance,
		isolate:       "isolate",
		isolateserver: "isolated",
		workdir:       absPath,
	}, nil
}

// Task is a description of the necessary inputs to isolate a task.
type Task struct {
	// BaseDir is the directory in which the files to be isolated reside.
	BaseDir string

	// ExtraVars is a map containing variable keys and values for the task.
	ExtraVars map[string]string

	// IsolateFile is the isolate file for this task.
	IsolateFile string

	// OsType is the OS on which the task will run.
	OsType string
}

// Validate returns an error if the Task is not valid.
func (t *Task) Validate() error {
	if t.BaseDir == "" {
		return fmt.Errorf("BaseDir is required.")
	}
	if t.IsolateFile == "" {
		return fmt.Errorf("IsolateFile is required.")
	}
	return nil
}

// IsolateTask uploads the necessary inputs for the task to the isolate server
// and returns the isolated hash.
func (c *Client) IsolateTask(ctx context.Context, task *Task) (string, error) {
	// Validation.
	if err := task.Validate(); err != nil {
		return "", err
	}

	// Setup.
	tmpDir, err := ioutil.TempDir("", "isolate")
	if err != nil {
		return "", skerr.Wrapf(err, "failed to create temporary dir")
	}
	defer util.RemoveAll(tmpDir)

	// Isolate the tasks.
	jsonOutput := filepath.Join(tmpDir, "isolated.json")
	cmd := []string{
		c.isolate, "archive", "--verbose",
		"--cas-instance", c.casInstance,
		"--dump-json", jsonOutput,
		"--isolate", task.IsolateFile,
	}
	if task.OsType != "" {
		cmd = append(cmd, "--config-variable", "OS", task.OsType)
	}

	if len(task.ExtraVars) > 0 {
		extraVarsKeys := make([]string, 0, len(task.ExtraVars))
		for k := range task.ExtraVars {
			extraVarsKeys = append(extraVarsKeys, k)
		}
		sort.Strings(extraVarsKeys)
		for _, k := range extraVarsKeys {
			cmd = append(cmd, "--extra-variable", k, task.ExtraVars[k])
		}
	}
	baseDir, err := filepath.Abs(task.BaseDir)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	if _, err := exec.RunCwd(ctx, baseDir, cmd...); err != nil {
		return "", skerr.Wrap(err)
	}

	// Read the JSON output file and return the hash.
	b, err := ioutil.ReadFile(jsonOutput)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	var hashes map[string]string
	if err := json.Unmarshal(b, &hashes); err != nil {
		return "", skerr.Wrap(err)
	}
	// We only provided one task, so there should only be one key in the map.
	if len(hashes) != 1 {
		return "", skerr.Fmt("Expected 1 hash but got %d; output: %s", len(hashes), string(b))
	}
	for _, hash := range hashes {
		return hash, nil
	}
	return "", skerr.Fmt("Don't know how to read isolated output")
}
