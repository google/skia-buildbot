package specs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/periodic"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/types"
)

const (
	DEFAULT_TASK_SPEC_MAX_ATTEMPTS = types.DEFAULT_MAX_TASK_ATTEMPTS

	// The default JobSpec.Priority, when unspecified or invalid.
	DEFAULT_JOB_SPEC_PRIORITY = 0.5

	TASKS_CFG_FILE = "infra/bots/tasks.json"

	// Triggering configuration for jobs.

	// By default, all jobs trigger on any branch for which they are
	// defined.
	TRIGGER_ANY_BRANCH = ""
	// Run this job on the main branch only, even if it is defined on others.
	TRIGGER_MASTER_ONLY = git.DefaultBranch
	// Trigger this job every night.
	TRIGGER_NIGHTLY = periodic.TRIGGER_NIGHTLY
	// Don't trigger this job automatically. It will only be run when
	// explicitly triggered via a try job or a force trigger.
	TRIGGER_ON_DEMAND = "on demand"
	// Trigger this job weekly.
	TRIGGER_WEEKLY = periodic.TRIGGER_WEEKLY

	VARIABLE_SYNTAX = "<(%s)"

	VARIABLE_BUILDBUCKET_BUILD_ID = "BUILDBUCKET_BUILD_ID"
	VARIABLE_CODEREVIEW_SERVER    = "CODEREVIEW_SERVER"
	VARIABLE_ISSUE                = "ISSUE"
	VARIABLE_ISSUE_INT            = "ISSUE_INT"
	VARIABLE_ISSUE_SHORT          = "ISSUE_SHORT"
	VARIABLE_PATCH_REF            = "PATCH_REF"
	VARIABLE_PATCH_REPO           = "PATCH_REPO"
	VARIABLE_PATCH_STORAGE        = "PATCH_STORAGE"
	VARIABLE_PATCHSET             = "PATCHSET"
	VARIABLE_PATCHSET_INT         = "PATCHSET_INT"
	VARIABLE_REPO                 = "REPO"
	VARIABLE_REVISION             = "REVISION"
	VARIABLE_TASK_ID              = "TASK_ID"
	VARIABLE_TASK_NAME            = "TASK_NAME"
)

var (
	// CIPD packages which may be used in tasks.
	CIPD_PKGS_GIT_LINUX_AMD64   []*CipdPackage = cipd.PkgsGit[cipd.PlatformLinuxAmd64]
	CIPD_PKGS_GIT_LINUX_ARM64   []*CipdPackage = cipd.PkgsGit[cipd.PlatformLinuxArm64]
	CIPD_PKGS_GIT_MAC_AMD64     []*CipdPackage = cipd.PkgsGit[cipd.PlatformMacAmd64]
	CIPD_PKGS_GIT_WINDOWS_386   []*CipdPackage = cipd.PkgsGit[cipd.PlatformWindows386]
	CIPD_PKGS_GIT_WINDOWS_AMD64 []*CipdPackage = cipd.PkgsGit[cipd.PlatformWindowsAmd64]
	CIPD_PKGS_GOLDCTL                          = []*CipdPackage{cipd.MustGetPackage("skia/tools/goldctl/${platform}")}
	CIPD_PKGS_GSUTIL                           = []*CipdPackage{cipd.MustGetPackage("infra/gsutil")}
	CIPD_PKGS_ISOLATE                          = []*CipdPackage{
		cipd.MustGetPackage("infra/tools/luci/isolate/${platform}"),
		cipd.MustGetPackage("infra/tools/luci/isolated/${platform}"),
	}
	CIPD_PKGS_PYTHON  []*CipdPackage = cipd.PkgsPython
	CIPD_PKGS_KITCHEN                = append([]*CipdPackage{
		cipd.MustGetPackage("infra/tools/luci/kitchen/${platform}"),
		cipd.MustGetPackage("infra/tools/luci-auth/${platform}"),
	}, CIPD_PKGS_PYTHON...)

	// Variable placeholders; these are replaced with the actual value
	// at task triggering time.
	PLACEHOLDER_BUILDBUCKET_BUILD_ID = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_BUILDBUCKET_BUILD_ID)
	PLACEHOLDER_CODEREVIEW_SERVER    = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_CODEREVIEW_SERVER)
	PLACEHOLDER_ISSUE                = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_ISSUE)
	PLACEHOLDER_ISSUE_INT            = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_ISSUE_INT)
	PLACEHOLDER_ISSUE_SHORT          = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_ISSUE_SHORT)
	PLACEHOLDER_PATCH_REF            = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCH_REF)
	PLACEHOLDER_PATCH_REPO           = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCH_REPO)
	PLACEHOLDER_PATCH_STORAGE        = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCH_STORAGE)
	PLACEHOLDER_PATCHSET             = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCHSET)
	PLACEHOLDER_PATCHSET_INT         = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_PATCHSET_INT)
	PLACEHOLDER_REPO                 = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_REPO)
	PLACEHOLDER_REVISION             = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_REVISION)
	PLACEHOLDER_TASK_ID              = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_TASK_ID)
	PLACEHOLDER_TASK_NAME            = fmt.Sprintf(VARIABLE_SYNTAX, VARIABLE_TASK_NAME)
	PLACEHOLDER_ISOLATED_OUTDIR      = "${ISOLATED_OUTDIR}"

	PERIODIC_TRIGGERS = []string{TRIGGER_NIGHTLY, TRIGGER_WEEKLY}
)

// ErrorIsPermanent returns true if the given error cannot be recovered by
// retrying. In this case, we will never be able to process the TasksCfg,
// so we might as well cancel the jobs.
// TODO(borenet): This should probably be split into three different
// ErrorIsPermanent functions, in the syncer, isolate, and specs packages.
func ErrorIsPermanent(err error) bool {
	return (strings.Contains(err.Error(), "error: Failed to merge in the changes.") ||
		strings.Contains(err.Error(), "Failed to apply patch") ||
		strings.Contains(err.Error(), "failed to process isolate") ||
		strings.Contains(err.Error(), "Failed to read tasks cfg: could not parse file:") ||
		strings.Contains(err.Error(), "Invalid TasksCfg") ||
		strings.Contains(err.Error(), "The \"gclient_gn_args_from\" value must be in recursedeps") ||
		// This repo was moved, so attempts to sync it will always fail.
		strings.Contains(err.Error(), "https://skia.googlesource.com/third_party/libjpeg-turbo.git") ||
		strings.Contains(err.Error(), "no such file or directory"))
}

// ParseTasksCfg parses the given task cfg file contents and returns the config.
func ParseTasksCfg(contents string) (*TasksCfg, error) {
	var rv TasksCfg
	if err := json.Unmarshal([]byte(contents), &rv); err != nil {
		return nil, fmt.Errorf("Failed to read tasks cfg: could not parse file: %s\nContents:\n%s", err, string(contents))
	}
	if err := rv.Validate(); err != nil {
		return nil, err
	}

	return &rv, nil
}

// EncoderTasksCfg writes the TasksCfg to a byte slice.
func EncodeTasksCfg(cfg *TasksCfg) ([]byte, error) {
	// Encode the JSON config.
	enc, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	// The json package escapes HTML characters, which makes our output
	// much less readable. Replace the escape characters with the real
	// character.
	enc = bytes.Replace(enc, []byte("\\u003c"), []byte("<"), -1)

	// Add a newline to the end of the file. Most text editors add one, so
	// adding one here enables manual editing of the file, even though we'd
	// rather that not happen.
	enc = append(enc, []byte("\n")...)
	return enc, nil
}

// ReadTasksCfg reads the task cfg file from the given dir and returns it.
func ReadTasksCfg(repoDir string) (*TasksCfg, error) {
	contents, err := ioutil.ReadFile(path.Join(repoDir, TASKS_CFG_FILE))
	if err != nil {
		// A nonexistent tasks.json file is valid; return an empty config.
		if os.IsNotExist(err) {
			return &TasksCfg{}, nil
		}
		return nil, fmt.Errorf("Failed to read tasks cfg: could not read file: %s", err)
	}
	return ParseTasksCfg(string(contents))
}

// WriteTasksCfg writes the task cfg to the given repo.
func WriteTasksCfg(cfg *TasksCfg, repoDir string) error {
	enc, err := EncodeTasksCfg(cfg)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path.Join(repoDir, TASKS_CFG_FILE), enc, os.ModePerm)
}

// TasksCfg is a struct which describes all Swarming tasks for a repo at a
// particular commit.
type TasksCfg struct {
	// Jobs is a map whose keys are JobSpec names and values are JobSpecs
	// which describe sets of tasks to run.
	Jobs map[string]*JobSpec `json:"jobs"`

	// Tasks is a map whose keys are TaskSpec names and values are TaskSpecs
	// detailing the Swarming tasks which may be run.
	Tasks map[string]*TaskSpec `json:"tasks"`

	// CasSpecs is a map of named specifications for content-addressed inputs to
	// tasks. If CasSpecs is not empty, RBE-CAS will be used instead of Isolate.
	CasSpecs map[string]*CasSpec `json:"casSpecs,omitempty"`
}

// Copy returns a deep copy of the TasksCfg.
func (c *TasksCfg) Copy() *TasksCfg {
	jobs := make(map[string]*JobSpec, len(c.Jobs))
	for name, job := range c.Jobs {
		jobs[name] = job.Copy()
	}
	tasks := make(map[string]*TaskSpec, len(c.Tasks))
	for name, task := range c.Tasks {
		tasks[name] = task.Copy()
	}
	var casSpecs map[string]*CasSpec
	if c.CasSpecs != nil {
		casSpecs = make(map[string]*CasSpec, len(c.CasSpecs))
		for name, spec := range c.CasSpecs {
			casSpecs[name] = spec.Copy()
		}
	}
	return &TasksCfg{
		Jobs:     jobs,
		Tasks:    tasks,
		CasSpecs: casSpecs,
	}
}

// Validate returns an error if the TasksCfg is not valid.
func (c *TasksCfg) Validate() error {
	// Validate all tasks.
	usedCasSpecs := make(map[string]bool, len(c.CasSpecs))
	for name, t := range c.Tasks {
		if err := t.Validate(c); err != nil {
			return fmt.Errorf("Invalid TasksCfg: %s", err)
		}

		// Ensure that any CAS inputs to the task exist.
		if len(c.CasSpecs) > 0 && t.Isolate != "" {
			return fmt.Errorf("Invalid TasksCfg: Task %q uses isolated input instead of CasSpec.", name)
		}
		if t.CasSpec != "" {
			if name, ok := c.CasSpecs[t.CasSpec]; !ok {
				return fmt.Errorf("Invalid TasksCfg: Task %q references non-existent CasSpec %q", name, t.CasSpec)
			}
			usedCasSpecs[t.CasSpec] = true
		}
	}

	// Ensure all CasSpecs are used.
	if len(usedCasSpecs) != len(c.CasSpecs) {
		for casSpec := range c.CasSpecs {
			if _, ok := usedCasSpecs[casSpec]; !ok {
				return fmt.Errorf("CasSpec %q is not referenced by any task.", casSpec)
			}
		}
	}

	// Validate all jobs.
	for _, j := range c.Jobs {
		if err := j.Validate(); err != nil {
			return fmt.Errorf("Invalid TasksCfg: %s", err)
		}
	}
	// Ensure that the DAG is valid.
	if err := findCycles(c.Tasks, c.Jobs); err != nil {
		return fmt.Errorf("Invalid TasksCfg: %s", err)
	}

	return nil
}

// TaskSpec is a struct which describes a Swarming task to run.
// Be sure to add any new fields to the Copy() method.
type TaskSpec struct {
	// Caches are named Swarming caches which should be used for this task.
	Caches []*Cache `json:"caches,omitempty"`

	// CasSpec references a named input to the task from content-addressed
	// storage.
	CasSpec string `json:"casSpec,omitempty"`

	// CipdPackages are CIPD packages which should be installed for the task.
	CipdPackages []*CipdPackage `json:"cipd_packages,omitempty"`

	// Command is the command to run in the Swarming task. If not specified
	// here, it should be specified in the .isolate file.
	Command []string `json:"command,omitempty"`

	// Dependencies are names of other TaskSpecs for tasks which need to run
	// before this task.
	Dependencies []string `json:"dependencies,omitempty"`

	// Dimensions are Swarming bot dimensions which describe the type of bot
	// which may run this task.
	Dimensions []string `json:"dimensions"`

	// Environment is a set of environment variables needed by the task.
	Environment map[string]string `json:"environment,omitempty"`

	// EnvPrefixes are prefixes to add to environment variables for the task,
	// for example, adding directories to PATH. Keys are environment variable
	// names and values are multiple values to add for the variable.
	EnvPrefixes map[string][]string `json:"env_prefixes,omitempty"`

	// ExecutionTimeout is the maximum amount of time the task is allowed
	// to take.
	ExecutionTimeout time.Duration `json:"execution_timeout_ns,omitempty"`

	// Expiration is how long the task may remain in the pending state
	// before it is abandoned.
	Expiration time.Duration `json:"expiration_ns,omitempty"`

	// ExtraArgs are extra command-line arguments to pass to the task.
	ExtraArgs []string `json:"extra_args,omitempty"`

	// ExtraTags are extra tags to add to the Swarming task.
	ExtraTags map[string]string `json:"extra_tags,omitempty"`

	// Idempotent indicates that triggering this task with the same
	// parameters as previously triggered has no side effect and thus the
	// task may be de-duplicated.
	Idempotent bool `json:"idempotent,omitempty"`

	// IoTimeout is the maximum amount of time which the task may take to
	// communicate with the server.
	IoTimeout time.Duration `json:"io_timeout_ns,omitempty"`

	// Isolate is the name of the isolate file used by this task.
	Isolate string `json:"isolate,omitempty"`

	// MaxAttempts is the maximum number of attempts for this TaskSpec. If
	// zero, DEFAULT_TASK_SPEC_MAX_ATTEMPTS is used.
	MaxAttempts int `json:"max_attempts,omitempty"`

	// Outputs are files and/or directories to use as outputs for the task.
	// Paths are relative to the task workdir. No error occurs if any of
	// these is missing.
	Outputs []string `json:"outputs,omitempty"`

	// This field is ignored.
	Priority float64 `json:"priority,omitempty"`

	// ServiceAccount indicates the Swarming service account to use for the
	// task. If not specified, we will attempt to choose a suitable default.
	ServiceAccount string `json:"service_account,omitempty"`
}

// Validate ensures that the TaskSpec is defined properly.
func (t *TaskSpec) Validate(cfg *TasksCfg) error {
	// Ensure that CIPD packages are specified properly.
	for _, p := range t.CipdPackages {
		if p.Name == "" || p.Path == "" {
			return fmt.Errorf("CIPD packages must have a name, path, and version.")
		}
	}

	if len(t.Dimensions) == 0 {
		return fmt.Errorf("Task must have dimensions")
	}

	// Ensure that the dimensions are specified properly.
	for _, d := range t.Dimensions {
		split := strings.SplitN(d, ":", 2)
		if len(split) != 2 {
			return fmt.Errorf("Dimension %q does not contain a colon!", d)
		}
	}

	// Isolate file is required.
	if t.Isolate == "" && t.CasSpec == "" {
		return fmt.Errorf("Isolate file or CasSpec is required.")
	}
	// Isolate and CasSpec are mutually exclusive.
	if t.Isolate != "" && t.CasSpec != "" {
		return fmt.Errorf("Only one of Isolate or CasSpec may be supplied.")
	}

	return nil
}

// Copy returns a copy of the TaskSpec.
func (t *TaskSpec) Copy() *TaskSpec {
	var caches []*Cache
	if len(t.Caches) > 0 {
		cachesDup := make([]Cache, len(t.Caches))
		caches = make([]*Cache, 0, len(t.Caches))
		for i, c := range t.Caches {
			cachesDup[i] = *c
			caches = append(caches, &cachesDup[i])
		}
	}
	var cipdPackages []*CipdPackage
	if len(t.CipdPackages) > 0 {
		cipdPackages = make([]*CipdPackage, 0, len(t.CipdPackages))
		pkgs := make([]CipdPackage, len(t.CipdPackages))
		for i, p := range t.CipdPackages {
			pkgs[i] = *p
			cipdPackages = append(cipdPackages, &pkgs[i])
		}
	}
	cmd := util.CopyStringSlice(t.Command)
	deps := util.CopyStringSlice(t.Dependencies)
	dims := util.CopyStringSlice(t.Dimensions)
	environment := util.CopyStringMap(t.Environment)
	var envPrefixes map[string][]string
	if len(t.EnvPrefixes) > 0 {
		envPrefixes = make(map[string][]string, len(t.EnvPrefixes))
		for k, v := range t.EnvPrefixes {
			envPrefixes[k] = util.CopyStringSlice(v)
		}
	}
	extraArgs := util.CopyStringSlice(t.ExtraArgs)
	extraTags := util.CopyStringMap(t.ExtraTags)
	outputs := util.CopyStringSlice(t.Outputs)
	return &TaskSpec{
		Caches:           caches,
		CasSpec:          t.CasSpec,
		CipdPackages:     cipdPackages,
		Command:          cmd,
		Dependencies:     deps,
		Dimensions:       dims,
		Environment:      environment,
		EnvPrefixes:      envPrefixes,
		ExecutionTimeout: t.ExecutionTimeout,
		Expiration:       t.Expiration,
		ExtraArgs:        extraArgs,
		ExtraTags:        extraTags,
		Idempotent:       t.Idempotent,
		IoTimeout:        t.IoTimeout,
		Isolate:          t.Isolate,
		MaxAttempts:      t.MaxAttempts,
		Outputs:          outputs,
		Priority:         t.Priority,
		ServiceAccount:   t.ServiceAccount,
	}
}

// Cache is a struct representing a named cache which is used by a task.
type Cache struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// CipdPackage is a struct representing a CIPD package which needs to be
// installed on a bot for a particular task.
// TODO(borenet): Are there any downsides to using an alias rather than a new
// type here?
type CipdPackage = cipd.Package

// JobSpec is a struct which describes a set of TaskSpecs to run as part of a
// larger effort.
type JobSpec struct {
	// Priority indicates the relative priority of the job, with 0 < p <= 1,
	// where higher values result in scheduling the job's tasks sooner. If
	// unspecified or outside this range, DEFAULT_JOB_SPEC_PRIORITY is used.
	// Each task derives its priority from the set of jobs that depend upon
	// it. A task's priority is
	//   1 - (1-<job1 priority>)(1-<job2 priority>)...(1-<jobN priority>)
	// A task at HEAD with a priority of 1 and a blamelist of 1 commit has
	// approximately the same score as a task at HEAD with a priority of 0.57
	// and a blamelist of 2 commits.
	// A backfill task with a priority of 1 that bisects a blamelist of 2
	// commits has the same score as another backfill task at the same
	// commit with a priority of 0.4 that bisects a blamelist of 4 commits.
	Priority float64 `json:"priority,omitempty"`
	// The names of TaskSpecs that are direct dependencies of this JobSpec.
	TaskSpecs []string `json:"tasks"`
	// One of the TRIGGER_* constants; see documentation above.
	Trigger string `json:"trigger,omitempty"`
}

// Validate returns an error if the JobSpec is not valid.
func (j *JobSpec) Validate() error {
	// We can't validate j.TaskSpecs here because we don't know which are
	// defined.  Therefore, that check needs to occur at a higher level.

	switch j.Trigger {
	case TRIGGER_ANY_BRANCH, TRIGGER_MASTER_ONLY, TRIGGER_NIGHTLY,
		TRIGGER_ON_DEMAND, TRIGGER_WEEKLY:
		break
	default:
		return fmt.Errorf("Invalid job trigger %q", j.Trigger)
	}
	return nil
}

// Copy returns a copy of the JobSpec.
func (j *JobSpec) Copy() *JobSpec {
	var taskSpecs []string
	if j.TaskSpecs != nil {
		taskSpecs = make([]string, len(j.TaskSpecs))
		copy(taskSpecs, j.TaskSpecs)
	}
	return &JobSpec{
		Priority:  j.Priority,
		TaskSpecs: taskSpecs,
		Trigger:   j.Trigger,
	}
}

// GetTaskSpecDAG returns a map describing all of the dependencies of the
// JobSpec. Its keys are TaskSpec names and values are TaskSpec names upon
// which the keys depend.
func (j *JobSpec) GetTaskSpecDAG(cfg *TasksCfg) (map[string][]string, error) {
	rv := map[string][]string{}
	var visit func(string) error
	visit = func(name string) error {
		if _, ok := rv[name]; ok {
			return nil
		}
		spec, ok := cfg.Tasks[name]
		if !ok {
			return fmt.Errorf("No such task: %s", name)
		}
		deps := util.CopyStringSlice(spec.Dependencies)
		if deps == nil {
			deps = []string{}
		}
		rv[name] = deps
		for _, t := range deps {
			if err := visit(t); err != nil {
				return err
			}
		}
		return nil
	}

	for _, t := range j.TaskSpecs {
		if err := visit(t); err != nil {
			return nil, err
		}
	}
	return rv, nil
}

// findCycles searches for cyclical dependencies in the task specs and returns
// an error if any are found. Also ensures that all task specs are reachable
// from at least one job spec and that all jobs specs' dependencies are valid.
func findCycles(tasks map[string]*TaskSpec, jobs map[string]*JobSpec) error {
	// Create vertex objects with metadata for the depth-first search.
	type vertex struct {
		active  bool
		name    string
		ts      *TaskSpec
		visited bool
	}
	vertices := make(map[string]*vertex, len(tasks))
	for name, t := range tasks {
		vertices[name] = &vertex{
			active:  false,
			name:    name,
			ts:      t,
			visited: false,
		}
	}

	// visit performs a depth-first search of the graph, starting at v.
	var visit func(*vertex) error
	visit = func(v *vertex) error {
		v.active = true
		v.visited = true
		for _, dep := range v.ts.Dependencies {
			e := vertices[dep]
			if e == nil {
				return fmt.Errorf("Task %q has unknown task %q as a dependency.", v.name, dep)
			}
			if !e.visited {
				if err := visit(e); err != nil {
					return err
				}
			} else if e.active {
				return fmt.Errorf("Found a circular dependency involving %q and %q", v.name, e.name)
			}
		}
		v.active = false
		return nil
	}

	// Perform a DFS, starting at each of the jobs' dependencies.
	for jobName, j := range jobs {
		for _, d := range j.TaskSpecs {
			v, ok := vertices[d]
			if !ok {
				return fmt.Errorf("Job %q has unknown task %q as a dependency.", jobName, d)
			}
			if !v.visited {
				if err := visit(v); err != nil {
					return err
				}
			}
		}
	}

	// If any vertices have not been visited, then there are tasks which
	// no job has as a dependency. Report an error.
	for _, v := range vertices {
		if !v.visited {
			return fmt.Errorf("Task %q is not reachable by any Job!", v.name)
		}
	}
	return nil
}

// CasSpec describes a set of task inputs in content-addressed storage.
type CasSpec struct {
	Root     string   `json:"root,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
	Digest   string   `json:"digest,omitempty"`
}

// Copy returns a deep copy of the CasSpec.
func (s *CasSpec) Copy() *CasSpec {
	return &CasSpec{
		Root:     s.Root,
		Paths:    util.CopyStringSlice(s.Paths),
		Excludes: util.CopyStringSlice(s.Excludes),
		Digest:   s.Digest,
	}
}

// Validate returns an error if the CasSpec is invalid.
func (s *CasSpec) Validate() error {
	if s.Root == "" && len(s.Paths) == 0 {
		if _, _, err := rbe.StringToDigest(s.Digest); err != nil {
			return skerr.Wrap(err)
		}
		return nil
	}
	if (s.Root != "") != (len(s.Paths) > 0) {
		return skerr.Fmt("Root and Paths must be specified together.")
	}
	return nil
}
