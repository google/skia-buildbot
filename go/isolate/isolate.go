package isolate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"sort"

	"go.chromium.org/luci/common/isolated"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	DEFAULT_NAMESPACE          = "default-gzip"
	ISOLATE_SERVER_URL         = "https://isolateserver.appspot.com"
	ISOLATE_SERVER_URL_FAKE    = "fake"
	ISOLATE_SERVER_URL_PRIVATE = "https://chrome-isolated.appspot.com"
	ISOLATE_SERVER_URL_DEV     = "https://isolateserver-dev.appspot.com/"
	ISOLATE_VERSION            = 1
	TASK_ID_TMPL               = "task_%s"
)

var (
	isolatedHashRegexpPattern = fmt.Sprintf("([a-f0-9]{40})\\s+.*(%s)\\.isolated$", fmt.Sprintf(TASK_ID_TMPL, "\\d+"))
	isolatedHashRegexp        = regexp.MustCompile(isolatedHashRegexpPattern)
)

// Client is a Skia-specific wrapper around the Isolate executable.
type Client struct {
	casInstance        string
	isolate            string
	isolateserver      string
	isolateServerURL   string
	workdir            string
	serviceAccountJSON string
}

// NewClient returns a Client instance which expects to find the "isolate" and
// "isolated" binaries in PATH. Typically they should be obtained via CIPD.
func NewClient(workdir, isolateServerURL, casInstance string) (*Client, error) {
	if workdir == "" {
		return nil, skerr.Fmt("workdir is required")
	}
	if (isolateServerURL == "" && casInstance == "") || (isolateServerURL != "" && casInstance != "") {
		return nil, skerr.Fmt("Exactly one of isolateServerURL or casInstance is required")
	}
	absPath, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	return &Client{
		casInstance:      casInstance,
		isolate:          "isolate",
		isolateserver:    "isolated",
		isolateServerURL: isolateServerURL,
		workdir:          absPath,
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

// writeIsolatedGenJson writes a temporary .isolated.gen.json file for the task.
func writeIsolatedGenJson(t *Task, genJsonFile, isolatedFile string) error {
	if err := t.Validate(); err != nil {
		return skerr.Wrap(err)
	}
	isolateFile, err := filepath.Abs(t.IsolateFile)
	if err != nil {
		return skerr.Wrap(err)
	}
	args := []string{
		"--isolate", isolateFile,
		"--isolated", isolatedFile,
	}
	if t.OsType != "" {
		args = append(args, "--config-variable", "OS", t.OsType)
	}

	if len(t.ExtraVars) > 0 {
		extraVarsKeys := make([]string, 0, len(t.ExtraVars))
		for k := range t.ExtraVars {
			extraVarsKeys = append(extraVarsKeys, k)
		}
		sort.Strings(extraVarsKeys)
		for _, k := range extraVarsKeys {
			args = append(args, "--extra-variable", k, t.ExtraVars[k])
		}
	}
	baseDir, err := filepath.Abs(t.BaseDir)
	if err != nil {
		return skerr.Wrap(err)
	}
	gen := struct {
		Version int      `json:"version"`
		Dir     string   `json:"dir"`
		Args    []string `json:"args"`
	}{
		Version: ISOLATE_VERSION,
		Dir:     baseDir,
		Args:    args,
	}
	err = util.WithWriteFile(genJsonFile, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(&gen)
	})
	return skerr.Wrap(err)
}

// isolateFile is a struct representing the contents of a .isolate file.
// TODO(borenet): Can we use something from go.chromium.org/luci/client/isolate?
type isolateFile struct {
	Command  []string
	Files    []string
	Includes []string
}

// Encode writes the encoded isolateFile into the given io.Writer.
func (f *isolateFile) Encode(w io.Writer) error {
	s := "{\n"
	if f.Includes != nil && len(f.Includes) > 0 {
		s += "  'includes': [\n"
		for _, inc := range f.Includes {
			s += fmt.Sprintf("    '%s',\n", inc)
		}
		s += "],\n"
	}
	s += "  'variables': {\n"
	if f.Command != nil && len(f.Command) > 0 {
		s += "    'command': [\n"
		for _, c := range f.Command {
			s += fmt.Sprintf("      '%s',\n", c)
		}
		s += "    ],\n"
	}
	if f.Files != nil && len(f.Files) > 0 {
		s += "    'files': [\n"
		for _, p := range f.Files {
			s += fmt.Sprintf("      '%s',\n", p)
		}
		s += "    ],\n"
	}
	s += "  },\n}"
	b := []byte(s)
	n, err := w.Write(b)
	if err != nil {
		return err
	}
	if n != len(b) {
		return fmt.Errorf("Failed to write all bytes.")
	}
	return nil
}

// Copy the Isolated.
func CopyIsolated(iso *isolated.Isolated) *isolated.Isolated {
	if iso == nil {
		return nil
	}
	var files map[string]isolated.File
	if iso.Files != nil {
		files = make(map[string]isolated.File, len(iso.Files))
		for k, v := range iso.Files {
			var link *string
			if v.Link != nil {
				linkVal := *v.Link
				link = &linkVal
			}
			var mode *int
			if v.Mode != nil {
				modeVal := *v.Mode
				mode = &modeVal
			}
			var size *int64
			if v.Size != nil {
				sizeVal := *v.Size
				size = &sizeVal
			}
			files[k] = isolated.File{
				Digest: v.Digest,
				Link:   link,
				Mode:   mode,
				Size:   size,
				Type:   v.Type,
			}
		}
	}
	var includes isolated.HexDigests
	if iso.Includes != nil {
		includes = make([]isolated.HexDigest, len(iso.Includes))
		copy(includes, iso.Includes)
	}
	return &isolated.Isolated{
		Algo:     iso.Algo,
		Files:    files,
		Includes: includes,
		Version:  iso.Version,
	}
}

// readIsolatedFile reads the given isolated file.
func readIsolatedFile(filepath string) (*isolated.Isolated, error) {
	var iso isolated.Isolated
	if err := util.WithReadFile(filepath, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&iso)
	}); err != nil {
		return nil, err
	}
	return &iso, nil
}

// writeIsolatedFile writes the given isolated file.
func writeIsolatedFile(filepath string, i *isolated.Isolated) error {
	return util.WithWriteFile(filepath, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(i)
	})
}

// batchArchiveTasks runs `isolate batcharchive` for the tasks.
func (c *Client) batchArchiveTasks(ctx context.Context, genJsonFiles []string, jsonOutput string) error {
	cmd := []string{
		c.isolate, "batcharchive", "--verbose",
	}
	if c.casInstance != "" {
		cmd = append(cmd, "--cas-instance", c.casInstance)
	} else {
		cmd = append(cmd, "--isolate-server", c.isolateServerURL)
	}
	if c.serviceAccountJSON != "" {
		cmd = append(cmd, "--service-account-json", c.serviceAccountJSON)
	}
	if jsonOutput != "" {
		cmd = append(cmd, "--dump-json", jsonOutput)
	}
	cmd = append(cmd, genJsonFiles...)
	output, err := exec.RunCwd(ctx, c.workdir, cmd...)
	if err != nil {
		return fmt.Errorf("Failed to run isolate: %s\nOutput:\n%s", err, output)
	}
	sklog.Infof("Isolate output: %s", output)
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

	// Write the .isolated.gen.json files.
	taskId := fmt.Sprintf(TASK_ID_TMPL, "0")
	genJsonFile := filepath.Join(tmpDir, fmt.Sprintf("%s.isolated.gen.json", taskId))
	isolatedFile := filepath.Join(tmpDir, fmt.Sprintf("%s.isolated", taskId))
	if err := writeIsolatedGenJson(task, genJsonFile, isolatedFile); err != nil {
		return "", err
	}

	// Isolate the tasks.
	jsonOutput := filepath.Join(tmpDir, "isolated.json")
	if err := c.batchArchiveTasks(ctx, []string{genJsonFile}, jsonOutput); err != nil {
		return "", err
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

// DownloadIsolateHash downloads the specified isolate hash into the specified output dir.
// downloadedFileList is the name of a file in the output dir into which the full list of
// downloaded files will be written to.
func (c *Client) DownloadIsolateHash(ctx context.Context, isolateHash, outputDir, downloadedFileList string) error {
	cmd := []string{
		c.isolateserver, "download", "--verbose",
		"--isolated", isolateHash,
		"--output-dir", outputDir,
		"--output-files", filepath.Join(outputDir, downloadedFileList),
	}
	if c.casInstance != "" {
		cmd = append(cmd, "--cas-instance", c.casInstance)
	} else {
		cmd = append(cmd, "--isolate-server", c.isolateServerURL)
	}
	if c.serviceAccountJSON != "" {
		cmd = append(cmd, "--service-account-json", c.serviceAccountJSON)
	}
	output, err := exec.RunCwd(ctx, c.workdir, cmd...)
	if err != nil {
		return fmt.Errorf("Failed to download isolate hash %s: %s\nOutput:\n%s", isolateHash, err, output)
	}
	return nil
}
