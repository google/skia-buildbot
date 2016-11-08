package scheduling

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"path"
	"strings"
	"time"

	swarming_api "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/specs"
)

// taskCandidate is a struct used for determining which tasks to schedule.
type taskCandidate struct {
	Commits        []string
	IsolatedInput  string
	IsolatedHashes []string
	JobCreated     time.Time
	ParentTaskIds  []string
	RetryOf        string
	Score          float64
	StealingFromId string
	db.TaskKey
	TaskSpec *specs.TaskSpec
}

// Copy returns a copy of the taskCandidate.
func (c *taskCandidate) Copy() *taskCandidate {
	return &taskCandidate{
		Commits:        util.CopyStringSlice(c.Commits),
		IsolatedInput:  c.IsolatedInput,
		IsolatedHashes: util.CopyStringSlice(c.IsolatedHashes),
		JobCreated:     c.JobCreated,
		ParentTaskIds:  util.CopyStringSlice(c.ParentTaskIds),
		RetryOf:        c.RetryOf,
		Score:          c.Score,
		StealingFromId: c.StealingFromId,
		TaskKey:        c.TaskKey.Copy(),
		TaskSpec:       c.TaskSpec.Copy(),
	}
}

// MakeId generates a string ID for the taskCandidate.
func (c *taskCandidate) MakeId() string {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&c.TaskKey); err != nil {
		panic(fmt.Sprintf("Failed to GOB encode TaskKey: %s", err))
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return fmt.Sprintf("taskCandidate|%s", b64)
}

// parseId generates taskCandidate information from the ID.
func parseId(id string) (db.TaskKey, error) {
	var rv db.TaskKey
	split := strings.Split(id, "|")
	if len(split) != 2 {
		return rv, fmt.Errorf("Invalid ID, not enough parts: %q", id)
	}
	if split[0] != "taskCandidate" {
		return rv, fmt.Errorf("Invalid ID, no 'taskCandidate' prefix: %q", id)
	}
	b, err := base64.StdEncoding.DecodeString(split[1])
	if err != nil {
		return rv, fmt.Errorf("Failed to base64 decode ID: %s", err)
	}
	if err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(&rv); err != nil {
		return rv, fmt.Errorf("Failed to GOB decode ID: %s", err)
	}
	return rv, nil
}

// MakeTask instantiates a db.Task from the taskCandidate.
func (c *taskCandidate) MakeTask() *db.Task {
	commits := make([]string, len(c.Commits))
	copy(commits, c.Commits)
	parentTaskIds := make([]string, len(c.ParentTaskIds))
	copy(parentTaskIds, c.ParentTaskIds)
	return &db.Task{
		Commits:       commits,
		Id:            "", // Filled in when the task is inserted into the DB.
		ParentTaskIds: parentTaskIds,
		RetryOf:       c.RetryOf,
		TaskKey:       c.TaskKey.Copy(),
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

// getPatchStorage returns "gerrit" or "rietveld" based on the Server URL.
func getPatchStorage(server string) string {
	if server == "" {
		return ""
	}
	if strings.Contains(server, "codereview.chromium") {
		return "rietveld"
	}
	return "gerrit"
}

// replaceVars replaces variable names with their values in a given string.
func replaceVars(c *taskCandidate, s string) string {
	issueShort := ""
	if len(c.Issue) < specs.ISSUE_SHORT_LENGTH {
		issueShort = c.Issue
	} else {
		issueShort = c.Issue[len(c.Issue)-specs.ISSUE_SHORT_LENGTH:]
	}
	replacements := map[string]string{
		specs.VARIABLE_CODEREVIEW_SERVER: c.Server,
		specs.VARIABLE_ISSUE:             c.Issue,
		specs.VARIABLE_ISSUE_SHORT:       issueShort,
		specs.VARIABLE_PATCH_STORAGE:     getPatchStorage(c.Server),
		specs.VARIABLE_PATCHSET:          c.Patchset,
		specs.VARIABLE_REPO:              c.Repo,
		specs.VARIABLE_REVISION:          c.Revision,
		specs.VARIABLE_TASK_NAME:         c.Name,
	}
	for k, v := range replacements {
		s = strings.Replace(s, fmt.Sprintf(specs.VARIABLE_SYNTAX, k), v, -1)
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
				Version:     p.Version,
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

	expirationSecs := int64(c.TaskSpec.Expiration.Seconds())
	if expirationSecs == int64(0) {
		expirationSecs = int64(swarming.RECOMMENDED_EXPIRATION.Seconds())
	}
	executionTimeoutSecs := int64(c.TaskSpec.ExecutionTimeout.Seconds())
	if executionTimeoutSecs == int64(0) {
		executionTimeoutSecs = int64(swarming.RECOMMENDED_HARD_TIMEOUT.Seconds())
	}
	ioTimeoutSecs := int64(c.TaskSpec.IoTimeout.Seconds())
	if ioTimeoutSecs == int64(0) {
		ioTimeoutSecs = int64(swarming.RECOMMENDED_IO_TIMEOUT.Seconds())
	}
	return &swarming_api.SwarmingRpcsNewTaskRequest{
		ExpirationSecs: expirationSecs,
		Name:           c.Name,
		Priority:       int64(100.0 * c.TaskSpec.Priority),
		Properties: &swarming_api.SwarmingRpcsTaskProperties{
			CipdInput:            cipdInput,
			Dimensions:           dims,
			Env:                  env,
			ExecutionTimeoutSecs: executionTimeoutSecs,
			ExtraArgs:            extraArgs,
			InputsRef: &swarming_api.SwarmingRpcsFilesRef{
				Isolated:       c.IsolatedInput,
				Isolatedserver: isolate.ISOLATE_SERVER_URL,
				Namespace:      isolate.DEFAULT_NAMESPACE,
			},
			IoTimeoutSecs: ioTimeoutSecs,
		},
		Tags: db.TagsForTask(c.Name, id, c.TaskSpec.Priority, c.RepoState, c.RetryOf, dimsMap, c.ForcedJobId, c.ParentTaskIds),
		User: "skia-task-scheduler",
	}
}

// allDepsMet determines whether all dependencies for the given task candidate
// have been satisfied, and if so, returns a map of whose keys are task IDs and
// values are their isolated outputs.
func (c *taskCandidate) allDepsMet(cache db.TaskCache) (bool, map[string]string, error) {
	rv := make(map[string]string, len(c.TaskSpec.Dependencies))
	for _, depName := range c.TaskSpec.Dependencies {
		key := c.TaskKey.Copy()
		key.Name = depName
		byKey, err := cache.GetTasksByKey(&key)
		if err != nil {
			return false, nil, err
		}
		ok := false
		for _, t := range byKey {
			if t.Done() && t.Success() && t.IsolatedOutput != "" {
				rv[t.Id] = t.IsolatedOutput
				ok = true
				break
			}
		}
		if !ok {
			return false, nil, nil
		}
	}
	return true, rv, nil
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
