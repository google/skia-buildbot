package scheduling

import (
	"fmt"
	"path"
	"strings"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/task_scheduler/go/db"
)

const (
	VARIABLE_SYNTAX = "<(%s)"

	VARIABLE_REPO      = "REPO"
	VARIABLE_REVISION  = "REVISION"
	VARIABLE_TASK_NAME = "TASK_NAME"
)

// taskCandidate is a struct used for determining which tasks to schedule.
type taskCandidate struct {
	Commits        []string
	IsolatedInput  string
	IsolatedHashes []string
	Name           string
	Repo           string
	Revision       string
	Score          float64
	StealingFromId string
	TaskSpec       *TaskSpec
}

// Copy returns a copy of the taskCandidate.
func (c *taskCandidate) Copy() *taskCandidate {
	commits := make([]string, len(c.Commits))
	copy(commits, c.Commits)
	isolatedHashes := make([]string, len(c.IsolatedHashes))
	copy(isolatedHashes, c.IsolatedHashes)
	return &taskCandidate{
		Commits:        commits,
		IsolatedInput:  c.IsolatedInput,
		IsolatedHashes: isolatedHashes,
		Name:           c.Name,
		Repo:           c.Repo,
		Revision:       c.Revision,
		Score:          c.Score,
		StealingFromId: c.StealingFromId,
		TaskSpec:       c.TaskSpec.Copy(),
	}
}

// MakeId generates a string ID for the taskCandidate.
func (c *taskCandidate) MakeId() string {
	return fmt.Sprintf("taskCandidate|%s|%s|%s", c.Repo, c.Name, c.Revision)
}

// ParseId generates taskCandidate information from the ID.
func parseId(id string) (string, string, string, error) {
	split := strings.Split(id, "|")
	if len(split) != 4 {
		return "", "", "", fmt.Errorf("Invalid ID: %q", id)
	}
	if split[0] != "taskCandidate" {
		return "", "", "", fmt.Errorf("Invalid ID: %q", id)
	}
	for _, s := range split[1:] {
		if s == "" {
			return "", "", "", fmt.Errorf("Invalid ID: %q", id)
		}
	}
	return split[1], split[2], split[3], nil
}

// MakeTask instantiates a db.Task from the taskCandidate.
func (c *taskCandidate) MakeTask() *db.Task {
	commits := make([]string, len(c.Commits))
	copy(commits, c.Commits)
	return &db.Task{
		Commits:  commits,
		Id:       "", // Filled in when the task is inserted into the DB.
		Name:     c.Name,
		Repo:     c.Repo,
		Revision: c.Revision,
	}
}

// MakeIsolateTask creates an isolate.Task from this taskCandidate.
func (c *taskCandidate) MakeIsolateTask(infraBotsDir, baseDir string) *isolate.Task {
	return &isolate.Task{
		BaseDir:     baseDir,
		Blacklist:   isolate.DEFAULT_BLACKLIST,
		Deps:        c.IsolatedHashes,
		IsolateFile: path.Join(infraBotsDir, c.TaskSpec.Isolate),
		OsType:      "linux", // TODO(borenet)
	}
}

// replaceVars replaces variable names with their values in a given string.
func replaceVars(c *taskCandidate, s string) string {
	replacements := map[string]string{
		VARIABLE_REPO:      c.Repo,
		VARIABLE_REVISION:  c.Revision,
		VARIABLE_TASK_NAME: c.Name,
	}
	for k, v := range replacements {
		s = strings.Replace(s, fmt.Sprintf(VARIABLE_SYNTAX, k), v, -1)
	}
	return s
}

// MakeTaskRequest creates a SwarmingRpcsNewTaskRequest object from the taskCandidate.
func (c *taskCandidate) MakeTaskRequest(id string) *swarming_api.SwarmingRpcsNewTaskRequest {
	var cipdInput *swarming_api.SwarmingRpcsCipdInput
	if len(c.TaskSpec.CipdPackages) > 0 {
		cipdInput = &swarming_api.SwarmingRpcsCipdInput{
			Packages: make([]*swarming_api.SwarmingRpcsCipdPackage, 0, len(c.TaskSpec.CipdPackages)),
		}
		for _, p := range c.TaskSpec.CipdPackages {
			cipdInput.Packages = append(cipdInput.Packages, &swarming_api.SwarmingRpcsCipdPackage{
				PackageName: p.Name,
				Path:        p.Path,
				Version:     fmt.Sprintf("%d", p.Version),
			})
		}
	}

	dims := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(c.TaskSpec.Dimensions))
	dimsMap := make(map[string]string, len(c.TaskSpec.Dimensions))
	for _, d := range c.TaskSpec.Dimensions {
		split := strings.SplitN(d, ":", 2)
		key := split[0]
		val := split[1]
		dims = append(dims, &swarming_api.SwarmingRpcsStringPair{
			Key:   key,
			Value: val,
		})
		dimsMap[key] = val
	}

	var env []*swarming_api.SwarmingRpcsStringPair
	if len(c.TaskSpec.Environment) > 0 {
		env = make([]*swarming_api.SwarmingRpcsStringPair, 0, len(c.TaskSpec.Environment))
		for k, v := range c.TaskSpec.Environment {
			env = append(env, &swarming_api.SwarmingRpcsStringPair{
				Key:   k,
				Value: v,
			})
		}
	}

	extraArgs := make([]string, 0, len(c.TaskSpec.ExtraArgs))
	for _, arg := range c.TaskSpec.ExtraArgs {
		extraArgs = append(extraArgs, replaceVars(c, arg))
	}

	return &swarming_api.SwarmingRpcsNewTaskRequest{
		ExpirationSecs: int64(swarming.RECOMMENDED_EXPIRATION.Seconds()),
		Name:           c.Name,
		Priority:       int64(100.0 * c.TaskSpec.Priority),
		Properties: &swarming_api.SwarmingRpcsTaskProperties{
			CipdInput:            cipdInput,
			Dimensions:           dims,
			Env:                  env,
			ExecutionTimeoutSecs: int64(swarming.RECOMMENDED_HARD_TIMEOUT.Seconds()),
			ExtraArgs:            extraArgs,
			InputsRef: &swarming_api.SwarmingRpcsFilesRef{
				Isolated:       c.IsolatedInput,
				Isolatedserver: isolate.ISOLATE_SERVER_URL,
				Namespace:      isolate.DEFAULT_NAMESPACE,
			},
			IoTimeoutSecs: int64(swarming.RECOMMENDED_IO_TIMEOUT.Seconds()),
		},
		Tags: db.TagsForTask(c.Name, id, c.TaskSpec.Priority, c.Repo, c.Revision, dimsMap),
		User: "skia-task-scheduler",
	}
}

// allDepsMet determines whether all dependencies for the given task candidate
// have been satisfied, and if so, returns their isolated outputs.
func (c *taskCandidate) allDepsMet(cache db.TaskCache) (bool, []string, error) {
	isolatedHashes := make([]string, 0, len(c.TaskSpec.Dependencies))
	for _, depName := range c.TaskSpec.Dependencies {
		d, err := cache.GetTaskForCommit(c.Repo, c.Revision, depName)
		if err != nil {
			return false, nil, err
		}
		if d == nil {
			return false, nil, nil
		}
		if !d.Done() || !d.Success() || d.IsolatedOutput == "" {
			return false, nil, nil
		}
		isolatedHashes = append(isolatedHashes, d.IsolatedOutput)
	}
	return true, isolatedHashes, nil
}

// taskCandidateSlice is an alias used for sorting a slice of taskCandidates.
type taskCandidateSlice []*taskCandidate

func (s taskCandidateSlice) Len() int { return len(s) }
func (s taskCandidateSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s taskCandidateSlice) Less(i, j int) bool {
	return s[i].Score > s[j].Score // candidates sort in decreasing order.
}
