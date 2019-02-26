package isolate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	"go.chromium.org/luci/common/isolated"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	DEFAULT_NAMESPACE          = "default-gzip"
	ISOLATE_EXE_SHA1           = "9734e966a14f9e26f86e38a020fcd7584248d285"
	ISOLATESERVER_EXE_SHA1     = "f4715e284c74ead3a0a6d4928b557f3029b38774"
	ISOLATE_SERVER_URL         = "https://isolateserver.appspot.com"
	ISOLATE_SERVER_URL_FAKE    = "fake"
	ISOLATE_SERVER_URL_PRIVATE = "https://chrome-isolated.appspot.com"
	ISOLATE_VERSION            = 1
	GCS_BUCKET                 = "chromium-luci"
	GCS_SUBDIR                 = ""
	TASK_ID_TMPL               = "task_%s"
)

var (
	DEFAULT_BLACKLIST = []string{"*.pyc", ".git", "out", ".recipe_deps"}

	isolatedHashRegexpPattern = fmt.Sprintf("([a-f0-9]{40})\\s+.*(%s)\\.isolated$", fmt.Sprintf(TASK_ID_TMPL, "\\d+"))
	isolatedHashRegexp        = regexp.MustCompile(isolatedHashRegexpPattern)
)

// Client is a Skia-specific wrapper around the Isolate executable.
type Client struct {
	gcs                *gcs.DownloadHelper
	isolate            string
	isolateserver      string
	serverUrl          string
	workdir            string
	serviceAccountJSON string
}

// NewClient returns a Client instance that uses "--service-account-json" for
// it's isolate binary calls. This is required for servers that are not ip
// whitelisted in chrome-infra-auth/ip_whitelist.cfg.
func NewClientWithServiceAccount(workdir, server, serviceAccountJSON string) (*Client, error) {
	c, err := NewClient(workdir, server)
	if err != nil {
		return nil, err
	}
	c.serviceAccountJSON = serviceAccountJSON
	return c, nil
}

// NewClient returns a Client instance.
func NewClient(workdir, server string) (*Client, error) {
	client := httputils.DefaultClientConfig().Client()
	// By default, the storage client tries really hard to be authenticated
	// with the scopes ReadWrite. Since the isolate executables are public
	// links, this is unnecessay and, in fact, causes errors if the user
	// doesn't have access to the bucket.
	s, err := storage.NewClient(context.Background(), option.WithScopes(storage.ScopeReadOnly), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	absPath, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	c := &Client{
		gcs:           gcs.NewDownloadHelper(s, GCS_BUCKET, GCS_SUBDIR, workdir),
		isolate:       path.Join(workdir, "isolate"),
		isolateserver: path.Join(workdir, "isolateserver"),
		serverUrl:     server,
		workdir:       absPath,
	}
	if err := c.gcs.MaybeDownload("isolate", ISOLATE_EXE_SHA1); err != nil {
		return nil, fmt.Errorf("Unable to create isolate client; failed to download isolate binary: %s", err)
	}
	if err := c.gcs.MaybeDownload("isolateserver", ISOLATESERVER_EXE_SHA1); err != nil {
		return nil, fmt.Errorf("Unable to create isolate client; failed to download isolateserver binary: %s", err)
	}

	return c, nil
}

// Close should be called when finished using the Client.
func (c *Client) Close() error {
	return c.gcs.Close()
}

// ServerURL return the Isolate server URL.
func (c *Client) ServerURL() string {
	return c.serverUrl
}

// Task is a description of the necessary inputs to isolate a task.
type Task struct {
	// BaseDir is the directory in which the files to be isolated reside.
	BaseDir string

	// Blacklist is a list of patterns of files not to upload.
	Blacklist []string

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

// WriteIsolatedGenJson writes a temporary .isolated.gen.json file for the task.
func WriteIsolatedGenJson(t *Task, genJsonFile, isolatedFile string) error {
	if err := t.Validate(); err != nil {
		return err
	}
	isolateFile, err := filepath.Abs(t.IsolateFile)
	if err != nil {
		return err
	}
	args := []string{
		"--isolate", isolateFile,
		"--isolated", isolatedFile,
	}
	if t.OsType != "" {
		args = append(args, "--config-variable", "OS", t.OsType)
	}
	for _, b := range t.Blacklist {
		args = append(args, "--blacklist", b)
	}
	for k, v := range t.ExtraVars {
		args = append(args, "--extra-variable", k, v)
	}
	baseDir, err := filepath.Abs(t.BaseDir)
	if err != nil {
		return err
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
	f, err := os.Create(genJsonFile)
	if err != nil {
		return fmt.Errorf("Failed to create %s: %s", genJsonFile, err)
	}
	defer util.Close(f)
	if err := json.NewEncoder(f).Encode(&gen); err != nil {
		return fmt.Errorf("Failed to write %s: %s", genJsonFile, err)
	}
	return nil
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
	var ro *isolated.ReadOnlyValue
	if iso.ReadOnly != nil {
		rov := *iso.ReadOnly
		ro = &rov
	}
	return &isolated.Isolated{
		Algo:        iso.Algo,
		Command:     util.CopyStringSlice(iso.Command),
		Files:       files,
		Includes:    includes,
		ReadOnly:    ro,
		RelativeCwd: iso.RelativeCwd,
		Version:     iso.Version,
	}
}

// ReadIsolatedFile reads the given isolated file.
func ReadIsolatedFile(filepath string) (*isolated.Isolated, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer util.Close(f)
	var iso isolated.Isolated
	if err := json.NewDecoder(f).Decode(&iso); err != nil {
		return nil, err
	}
	return &iso, nil
}

// WriteIsolatedFile writes the given isolated file.
func WriteIsolatedFile(filepath string, i *isolated.Isolated) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(i); err != nil {
		defer util.Close(f)
		return err
	}
	return f.Close()
}

// BatchArchiveTasks runs `isolate batcharchive` for the tasks.
func (c *Client) BatchArchiveTasks(ctx context.Context, genJsonFiles []string, jsonOutput string) error {
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
		genJsonFile := path.Join(tmpDir, fmt.Sprintf("%s.isolated.gen.json", taskId))
		isolatedFile := path.Join(tmpDir, fmt.Sprintf("%s.isolated", taskId))
		if err := WriteIsolatedGenJson(t, genJsonFile, isolatedFile); err != nil {
			return nil, nil, err
		}
		genJsonFiles = append(genJsonFiles, genJsonFile)
		isolatedFilePaths = append(isolatedFilePaths, isolatedFile)
	}

	// Isolate the tasks.
	if err := c.BatchArchiveTasks(ctx, genJsonFiles, ""); err != nil {
		return nil, nil, err
	}

	// Read the isolated files and add any extra dependencies.
	isolatedFiles := make([]*isolated.Isolated, 0, len(isolatedFilePaths))
	for i, f := range isolatedFilePaths {
		t := tasks[i]
		iso, err := ReadIsolatedFile(f)
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
		if err := WriteIsolatedFile(filePath, isolatedFile); err != nil {
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
	for i, _ := range isolatedFiles {
		rv = append(rv, hashes[fmt.Sprintf(TASK_ID_TMPL, strconv.Itoa(i))])
	}
	return rv, nil
}
