package scheduling

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"sort"
	"strconv"
	"strings"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_scheduler/go/db/cache"
	"go.skia.org/infra/task_scheduler/go/specs"
	"go.skia.org/infra/task_scheduler/go/types"
)

// taskCandidate is a struct used for determining which tasks to schedule.
type taskCandidate struct {
	Attempt int `json:"attempt"`
	// NB: Because multiple Jobs may share a Task, the BuildbucketBuildId
	// could be inherited from any matching Job. Therefore, this should be
	// used for non-critical, informational purposes only.
	BuildbucketBuildId int64    `json:"buildbucketBuildId"`
	Commits            []string `json:"commits"`
	CasInput           string   `json:"casInput"`
	CasDigests         []string `json:"casDigests"`
	CasUsesIsolate     bool     `json:"casUsesIsolate"`
	// Jobs must be kept in sorted order; see AddJob.
	Jobs           []*types.Job `json:"jobs"`
	ParentTaskIds  []string     `json:"parentTaskIds"`
	RetryOf        string       `json:"retryOf"`
	Score          float64      `json:"score"`
	StealingFromId string       `json:"stealingFromId"`
	types.TaskKey
	TaskSpec    *specs.TaskSpec           `json:"taskSpec"`
	Diagnostics *taskCandidateDiagnostics `json:"diagnostics,omitempty"`
}

// CopyNoDiagnostics returns a copy of the taskCandidate, omitting the
// Diagnostics field.
func (c *taskCandidate) CopyNoDiagnostics() *taskCandidate {
	jobs := make([]*types.Job, len(c.Jobs))
	copy(jobs, c.Jobs)
	return &taskCandidate{
		Attempt:            c.Attempt,
		BuildbucketBuildId: c.BuildbucketBuildId,
		Commits:            util.CopyStringSlice(c.Commits),
		CasInput:           c.CasInput,
		CasDigests:         util.CopyStringSlice(c.CasDigests),
		CasUsesIsolate:     c.CasUsesIsolate,
		Jobs:               jobs,
		ParentTaskIds:      util.CopyStringSlice(c.ParentTaskIds),
		RetryOf:            c.RetryOf,
		Score:              c.Score,
		StealingFromId:     c.StealingFromId,
		TaskKey:            c.TaskKey.Copy(),
		TaskSpec:           c.TaskSpec.Copy(),
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
func parseId(id string) (types.TaskKey, error) {
	var rv types.TaskKey
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

// findJob locates job in c.Jobs and returns its index and true if found or the
// insertion index and false if not.
func (c *taskCandidate) findJob(job *types.Job) (int, bool) {
	idx := sort.Search(len(c.Jobs), func(i int) bool {
		return !c.Jobs[i].Created.Before(job.Created)
	})
	if idx >= len(c.Jobs) {
		return idx, false
	}
	for offset, j := range c.Jobs[idx:] {
		if j.Id == job.Id {
			return idx + offset, true
		}
		if j.Created.After(job.Created) {
			return idx + offset, false
		}
	}
	// This shouldn't happen, but golang doesn't know that.
	return len(c.Jobs), false
}

// HasJob returns true if job is a member of c.Jobs.
func (c *taskCandidate) HasJob(job *types.Job) bool {
	_, ok := c.findJob(job)
	return ok
}

// AddJob adds job to c.Jobs, unless already present.
func (c *taskCandidate) AddJob(job *types.Job) {
	idx, ok := c.findJob(job)
	if !ok {
		c.Jobs = append(c.Jobs, nil)
		copy(c.Jobs[idx+1:], c.Jobs[idx:])
		c.Jobs[idx] = job
	}
}

// MakeTask instantiates a types.Task from the taskCandidate.
func (c *taskCandidate) MakeTask() *types.Task {
	commits := make([]string, len(c.Commits))
	copy(commits, c.Commits)
	jobs := make([]string, 0, len(c.Jobs))
	for _, j := range c.Jobs {
		jobs = append(jobs, j.Id)
	}
	sort.Strings(jobs)
	parentTaskIds := make([]string, len(c.ParentTaskIds))
	copy(parentTaskIds, c.ParentTaskIds)
	maxAttempts := c.TaskSpec.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = specs.DEFAULT_TASK_SPEC_MAX_ATTEMPTS
	}
	return &types.Task{
		Attempt:       c.Attempt,
		Commits:       commits,
		Id:            "", // Filled in when the task is inserted into the DB.
		Jobs:          jobs,
		MaxAttempts:   maxAttempts,
		ParentTaskIds: parentTaskIds,
		RetryOf:       c.RetryOf,
		TaskKey:       c.TaskKey.Copy(),
	}
}

// getPatchStorage returns "gerrit" or "" based on the Server URL.
func getPatchStorage(server string) string {
	if server == "" {
		return ""
	}
	return "gerrit"
}

// replaceVars replaces variable names with their values in a given string.
func replaceVars(c *taskCandidate, s, taskId string) string {
	issueShort := ""
	if len(c.Issue) < types.ISSUE_SHORT_LENGTH {
		issueShort = c.Issue
	} else {
		issueShort = c.Issue[len(c.Issue)-types.ISSUE_SHORT_LENGTH:]
	}
	issueInt := c.Issue
	if issueInt == "" {
		issueInt = "0"
	}
	patchsetInt := c.Patchset
	if patchsetInt == "" {
		patchsetInt = "0"
	}
	replacements := map[string]string{
		specs.VARIABLE_BUILDBUCKET_BUILD_ID: strconv.FormatInt(c.BuildbucketBuildId, 10),
		specs.VARIABLE_CODEREVIEW_SERVER:    c.Server,
		specs.VARIABLE_ISSUE:                c.Issue,
		specs.VARIABLE_ISSUE_INT:            issueInt,
		specs.VARIABLE_ISSUE_SHORT:          issueShort,
		specs.VARIABLE_PATCH_REF:            c.RepoState.GetPatchRef(),
		specs.VARIABLE_PATCH_REPO:           c.PatchRepo,
		specs.VARIABLE_PATCH_STORAGE:        getPatchStorage(c.Server),
		specs.VARIABLE_PATCHSET:             c.Patchset,
		specs.VARIABLE_PATCHSET_INT:         patchsetInt,
		specs.VARIABLE_REPO:                 c.Repo,
		specs.VARIABLE_REVISION:             c.Revision,
		specs.VARIABLE_TASK_ID:              taskId,
		specs.VARIABLE_TASK_NAME:            c.Name,
	}
	for k, v := range replacements {
		s = strings.Replace(s, fmt.Sprintf(specs.VARIABLE_SYNTAX, k), v, -1)
	}
	return s
}

// MakeTaskRequest creates a SwarmingRpcsNewTaskRequest object from the taskCandidate.
func (c *taskCandidate) MakeTaskRequest(id, casInstance, isolateServer, pubSubTopic string) (*swarming_api.SwarmingRpcsNewTaskRequest, error) {
	var caches []*swarming_api.SwarmingRpcsCacheEntry
	if len(c.TaskSpec.Caches) > 0 {
		caches = make([]*swarming_api.SwarmingRpcsCacheEntry, 0, len(c.TaskSpec.Caches))
		for _, cache := range c.TaskSpec.Caches {
			caches = append(caches, &swarming_api.SwarmingRpcsCacheEntry{
				Name: cache.Name,
				Path: cache.Path,
			})
		}
	}
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

	var envPrefixes []*swarming_api.SwarmingRpcsStringListPair
	if len(c.TaskSpec.EnvPrefixes) > 0 {
		envPrefixes = make([]*swarming_api.SwarmingRpcsStringListPair, 0, len(c.TaskSpec.EnvPrefixes))
		for k, v := range c.TaskSpec.EnvPrefixes {
			envPrefixes = append(envPrefixes, &swarming_api.SwarmingRpcsStringListPair{
				Key:   k,
				Value: util.CopyStringSlice(v),
			})
		}
	}

	cmd := make([]string, 0, len(c.TaskSpec.Command))
	for _, arg := range c.TaskSpec.Command {
		cmd = append(cmd, replaceVars(c, arg, id))
	}

	extraArgs := make([]string, 0, len(c.TaskSpec.ExtraArgs))
	for _, arg := range c.TaskSpec.ExtraArgs {
		extraArgs = append(extraArgs, replaceVars(c, arg, id))
	}

	extraTags := make(map[string]string, len(c.TaskSpec.ExtraTags))
	for k, v := range c.TaskSpec.ExtraTags {
		extraTags[k] = replaceVars(c, v, id)
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
	outputs := util.CopyStringSlice(c.TaskSpec.Outputs)
	idempotent := c.TaskSpec.Idempotent
	if c.ForcedJobId != "" {
		// Don't allow deduplication for forced jobs.
		idempotent = false
	}
	req := &swarming_api.SwarmingRpcsNewTaskRequest{
		Name:           c.Name,
		Priority:       swarming.RECOMMENDED_PRIORITY,
		PubsubTopic:    fmt.Sprintf(swarming.PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, pubSubTopic),
		PubsubUserdata: id,
		ServiceAccount: c.TaskSpec.ServiceAccount,
		Tags:           types.TagsForTask(c.Name, id, c.Attempt, c.RepoState, c.RetryOf, dimsMap, c.ForcedJobId, c.ParentTaskIds, extraTags),
		TaskSlices: []*swarming_api.SwarmingRpcsTaskSlice{
			{
				ExpirationSecs: expirationSecs,
				Properties: &swarming_api.SwarmingRpcsTaskProperties{
					Caches:               caches,
					CipdInput:            cipdInput,
					Command:              cmd,
					Dimensions:           dims,
					Env:                  env,
					EnvPrefixes:          envPrefixes,
					ExecutionTimeoutSecs: executionTimeoutSecs,
					ExtraArgs:            extraArgs,
					Idempotent:           idempotent,
					IoTimeoutSecs:        ioTimeoutSecs,
					Outputs:              outputs,
				},
				WaitForCapacity: false,
			},
		},
		User: "skiabot@google.com",
	}
	if c.CasUsesIsolate {
		req.TaskSlices[0].Properties.InputsRef = &swarming_api.SwarmingRpcsFilesRef{
			Isolated:       c.CasInput,
			Isolatedserver: isolateServer,
			Namespace:      isolate.DEFAULT_NAMESPACE,
		}
	} else {
		hash, size, err := rbe.StringToDigest(c.CasInput)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		req.TaskSlices[0].Properties.CasInputRoot = &swarming_api.SwarmingRpcsCASReference{
			CasInstance: casInstance,
			Digest: &swarming_api.SwarmingRpcsDigest{
				Hash:            hash,
				SizeBytes:       size,
				ForceSendFields: []string{"SizeBytes"},
			},
		}
	}
	return req, nil
}

// allDepsMet determines whether all dependencies for the given task candidate
// have been satisfied, and if so, returns a map of whose keys are task IDs and
// values are their isolated outputs.
func (c *taskCandidate) allDepsMet(cache cache.TaskCache) (bool, map[string]string, error) {
	rv := make(map[string]string, len(c.TaskSpec.Dependencies))
	var missingDeps []string
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
			missingDeps = append(missingDeps, depName)
		}
	}
	if len(missingDeps) > 0 {
		c.GetDiagnostics().Filtering = &taskCandidateFilteringDiagnostics{
			UnmetDependencies: missingDeps,
		}
		return false, nil, nil
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

// taskCandidateDiagnostics stores info about why a candidate was not triggered.
type taskCandidateDiagnostics struct {
	// Filtering contains reasons that a candidate would be rejected before scoring and scheduling.
	// Not set if the candidate was not filtered out.
	Filtering *taskCandidateFilteringDiagnostics `json:"filtering,omitempty"`
	// Scoring contains intermediate results in the calculation of Score. Always set unless Filtering
	// is set.
	Scoring *taskCandidateScoringDiagnostics `json:"scoring,omitempty"`
	// Scheduling contains details about selecting candidates from the queue. Always set unless
	// Filtering is set.
	Scheduling *taskCandidateSchedulingDiagnostics `json:"scheduling,omitempty"`
	// Triggering contains detailed results of triggering tasks. Set only if this candidate was
	// selected to be triggered (Scheduling.Selected is true).
	Triggering *taskCandidateTriggeringDiagnostics `json:"triggering,omitempty"`
}

func (s *taskCandidate) GetDiagnostics() *taskCandidateDiagnostics {
	if s.Diagnostics == nil {
		s.Diagnostics = &taskCandidateDiagnostics{}
	}
	return s.Diagnostics
}

// taskCandidateFilteringDiagnostics contains information about a candidate rejected before scoring
// and scheduling. Normally exactly one field is set.
type taskCandidateFilteringDiagnostics struct {
	// Name of rule skipping this task.
	SkippedByRule string `json:"skippedByRule,omitempty"`
	// True if this task's revision is outside the scheduling window (but its Job has not yet been
	// flushed).
	RevisionTooOld bool `json:"revisionTooOld,omitempty"`
	// TaskId of a pending, running, or completed task with the same TaskKey.
	SupersededByTask string `json:"supersededByTask,omitempty"`
	// TaskIds of previous attempts; set when max attempts have been reached.
	PreviousAttempts []string `json:"previousAttempts,omitempty"`
	// Names of TaskSpec dependencies that have not completed.
	UnmetDependencies []string `json:"unmetDependencies,omitempty"`
}

// taskCandidateScoringDiagnostics contains intermediate results in the calculation of Score. For
// regular tasks (not forced or try jobs), all fields are set.
type taskCandidateScoringDiagnostics struct {
	// Priority calculated from all dependent Job priorities. (Note this is *not* the same as Score;
	// Priority is an input to scoring while Score is the output.)
	Priority float64 `json:"priority,omitempty"`
	// Hours since this candidate's earliest Job was created (only used for forced and try jobs).
	JobCreatedHours float64 `json:"jobCreatedHours,omitempty"`
	// Number of commits in this candidate's blamelist that previously were in Task's or candidate's
	// blamelist. (Note that the number of commits in this candidate's blamelist can be derived from
	// the Commits field.) Not set for forced or try jobs.
	StoleFromCommits int `json:"stoleFromCommits,omitempty"`
	// Base score. See doc for testednessIncrease in task_scheduler.go. Not set for forced or try
	// jobs.
	TestednessIncrease float64 `json:"testednessIncrease,omitempty"`
	// Multiplier to prioritize newer commits. Not set for forced or try jobs.
	TimeDecay float64 `json:"timeDecay,omitempty"`
}

// taskCandidateSchedulingDiagnostics contains information about matching tasks with bots.
type taskCandidateSchedulingDiagnostics struct {
	// True if SCHEDULING_LIMIT_PER_TASK_SPEC was reached for this task spec. The remaining fields
	// will not be set.
	OverSchedulingLimitPerTaskSpec bool `json:"overSchedulingLimitPerTaskSpec,omitempty"`
	// True if the candidate was skipped because its score was below the threshold. The remaining
	// fields will not be set.
	ScoreBelowThreshold bool `json:"scoreBelowThreshold,omitempty"`
	// True if no matching bots are available. (This is an explicit marker for len(MatchingBots) == 0
	// since we use the JSON omitempty option.)
	// This field also indicates whether NumHigherScoreSimilarCandidates and LastSimilarCandidate are
	// approximate (true) or exact (false). (When there are no matching bots available, it is not
	// possible to determine if the same bots would satisfy different sets of dimensions, e.g. CPU
	// tasks vs GPU tasks.)
	NoBotsAvailable bool `json:"noBotsAvailable,omitempty"`
	// The list of available bots that match this candidate's dimensions, regardless of other
	// candidates.
	MatchingBots []string `json:"matchingBots,omitempty"`
	// MatchingBots is non-empty: Count of candidates with a higher score that could have used one of
	// the bots that match this candidate's dimensions.
	// MatchingBots is empty: Count of candidates with a higher score that have the same dimensions
	// as this candidate.
	NumHigherScoreSimilarCandidates int `json:"numHigherScoreSimilarCandidates,omitempty"`
	// Lowest-score candidate included in NumHigherScoreSimilarCandidates, identified by the
	// candidate's TaskKey. In many cases, it is possible to identify all candidates included in
	// NumHigherScoreSimilarCandidates by following the chain of LastSimilarCandidate.
	LastSimilarCandidate *types.TaskKey `json:"lastSimilarCandidate,omitempty"`
	// True if this candidate has been selected to run.
	Selected bool `json:"selected,omitempty"`
}

// taskCandidateTriggeringDiagnostics contains information about triggering a Swarming task for this
// candidate.
type taskCandidateTriggeringDiagnostics struct {
	// Error message from isolating inputs.
	IsolateError string `json:"isolateError,omitempty"`
	// Error message from triggering the task.
	TriggerError string `json:"triggerError,omitempty"`
	// Task Scheduler ID of the triggered task. If an error occurs after assigning an ID, the Task may
	// not exist in the Task Scheduler DB.
	TaskId string `json:"taskId,omitempty"`
}
