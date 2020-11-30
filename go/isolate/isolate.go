package isolate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"go.chromium.org/luci/common/isolated"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/skerr"
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
	isolate            string
	isolateserver      string
	serverUrl          string
	workdir            string
	serviceAccountJSON string
}

// NewClient returns a Client instance which expects to find the "isolate" and
// "isolated" binaries in PATH. Typically they should be obtained via CIPD.
func NewClient(workdir, server string) (*Client, error) {
	if workdir == "" {
		return nil, skerr.Fmt("workdir is required")
	}
	if server == "" {
		return nil, skerr.Fmt("server is required")
	}
	absPath, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	return &Client{
		isolate:       "isolate",
		isolateserver: "isolated",
		serverUrl:     server,
		workdir:       absPath,
	}, nil
}

// NewClientWithServiceAccount returns a Client instance which uses
// "--service-account-json" for its isolate binary calls. This is required for
// servers that are not listed in the chrome-infra-auth bypass list.
func NewClientWithServiceAccount(workdir, server, serviceAccountJSON string) (*Client, error) {
	c, err := NewClient(workdir, server)
	if err != nil {
		return nil, err
	}
	c.serviceAccountJSON = serviceAccountJSON
	return c, nil
}

// ServerURL return the Isolate server URL.
func (c *Client) ServerURL() string {
	return c.serverUrl
}

// Task is a description of the necessary inputs to isolate a task.
type Task struct {
	// BaseDir is the directory in which the files to be isolated reside.
	BaseDir string

	// Deps is a list of isolated hashes upon which this task depends.
	Deps []string

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
		"--isolate-server", c.serverUrl,
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
	return nil
}

// IsolateTasks uploads the necessary inputs for the task to the isolate server
// and returns the isolated hashes.
func (c *Client) IsolateTasks(ctx context.Context, tasks []*Task) ([]string, []*isolated.Isolated, error) {
	// Validation.
	if len(tasks) == 0 {
		return []string{}, []*isolated.Isolated{}, nil
	}
	for _, t := range tasks {
		if err := t.Validate(); err != nil {
			return nil, nil, err
		}
	}

	// Setup.
	tmpDir, err := ioutil.TempDir("", "isolate")
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create temporary dir: %s", err)
	}
	defer util.RemoveAll(tmpDir)

	// Write the .isolated.gen.json files.
	genJsonFiles := make([]string, 0, len(tasks))
	isolatedFilePaths := make([]string, 0, len(tasks))
	for i, t := range tasks {
		taskId := fmt.Sprintf(TASK_ID_TMPL, strconv.Itoa(i))
		genJsonFile := filepath.Join(tmpDir, fmt.Sprintf("%s.isolated.gen.json", taskId))
		isolatedFile := filepath.Join(tmpDir, fmt.Sprintf("%s.isolated", taskId))
		if err := writeIsolatedGenJson(t, genJsonFile, isolatedFile); err != nil {
			return nil, nil, err
		}
		genJsonFiles = append(genJsonFiles, genJsonFile)
		isolatedFilePaths = append(isolatedFilePaths, isolatedFile)
	}

	// Isolate the tasks.
	if err := c.batchArchiveTasks(ctx, genJsonFiles, ""); err != nil {
		return nil, nil, err
	}

	// Read the isolated files and add any extra dependencies.
	isolatedFiles := make([]*isolated.Isolated, 0, len(isolatedFilePaths))
	for i, f := range isolatedFilePaths {
		t := tasks[i]
		iso, err := readIsolatedFile(f)
		if err != nil {
			return nil, nil, err
		}
		for _, dep := range t.Deps {
			iso.Includes = append(iso.Includes, isolated.HexDigest(dep))
		}
		isolatedFiles = append(isolatedFiles, iso)
	}
	hashes, err := c.ReUploadIsolatedFiles(ctx, isolatedFiles)
	if err != nil {
		return nil, nil, err
	}
	return hashes, isolatedFiles, err
}

// DownloadIsolateHash downloads the specified isolate hash into the specified output dir.
// downloadedFileList is the name of a file in the output dir into which the full list of
// downloaded files will be written to.
func (c *Client) DownloadIsolateHash(ctx context.Context, isolateHash, outputDir, downloadedFileList string) error {
	cmd := []string{
		c.isolateserver, "download", "--verbose",
		"--isolate-server", c.serverUrl,
		"--isolated", isolateHash,
		"--output-dir", outputDir,
		"--output-files", filepath.Join(outputDir, downloadedFileList),
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

// ReUploadIsolatedFiles re-uploads the given existing isolated files, eg. to add dependencies.
func (c *Client) ReUploadIsolatedFiles(ctx context.Context, isolatedFiles []*isolated.Isolated) ([]string, error) {
	// Setup.
	tmpDir, err := ioutil.TempDir("", "isolate")
	if err != nil {
		return nil, fmt.Errorf("Failed to create temporary dir: %s", err)
	}
	defer util.RemoveAll(tmpDir)

	// Re-upload the isolated files.
	isolatedFilePaths := make([]string, 0, len(isolatedFiles))
	for i, isolatedFile := range isolatedFiles {
		taskId := fmt.Sprintf(TASK_ID_TMPL, strconv.Itoa(i))
		filePath := filepath.Join(tmpDir, fmt.Sprintf("%s.isolated", taskId))
		isolatedFilePaths = append(isolatedFilePaths, filePath)
		if err := writeIsolatedFile(filePath, isolatedFile); err != nil {
			return nil, err
		}
	}

	cmd := []string{
		c.isolateserver, "archive", "--verbose",
		"--isolate-server", c.serverUrl,
	}
	if c.serviceAccountJSON != "" {
		cmd = append(cmd, "--service-account-json", c.serviceAccountJSON)
	}
	for _, f := range isolatedFilePaths {
		dirname, filename := path.Split(f)
		if runtime.GOOS == "windows" {
			// Win path prefixes seem to confuse isolate server.
			dirname = strings.TrimPrefix(dirname, `c:`)
			filename = strings.TrimPrefix(filename, `c:`)
		}
		cmd = append(cmd, "--files", fmt.Sprintf("%s:%s", dirname, filename))
	}
	output, err := exec.RunCwd(ctx, c.workdir, cmd...)
	if err != nil {
		return nil, fmt.Errorf("Failed to run isolate: %s\nOutput:\n%s", err, output)
	}

	// Parse isolated hash for each task from the output.
	hashes := map[string]string{}
	for _, line := range strings.Split(string(output), "\n") {
		m := isolatedHashRegexp.FindStringSubmatch(line)
		if m != nil {
			if len(m) != 3 {
				return nil, fmt.Errorf("Isolated output regexp returned invalid match: %v", m)
			}
			hashes[m[2]] = m[1]
		}
	}
	if len(hashes) != len(isolatedFiles) {
		return nil, fmt.Errorf("Ended up with an incorrect number of isolated hashes:\n%s", string(output))
	}
	rv := make([]string, 0, len(isolatedFiles))
	for i := range isolatedFiles {
		rv = append(rv, hashes[fmt.Sprintf(TASK_ID_TMPL, strconv.Itoa(i))])
	}
	return rv, nil
}
