// TODO(b/500974820): Reuse types from `pinpoint/proto/v1/service.pb.go`.
package internal

type TryJobCreateRequest struct {
	Name        string `json:"name"`
	BaseGitHash string `json:"base_git_hash"`
	// although "experiment" makes more sense in this context, the legacy Pinpoint API
	// explicitly defines the experiment commit as "end_git_hash" and defines
	// the experiment patch as "experiment_patch"
	EndGitHash      string `json:"end_git_hash"`
	BasePatch       string `json:"base_patch"`
	ExperimentPatch string `json:"experiment_patch"`
	Configuration   string `json:"configuration"`
	Benchmark       string `json:"benchmark"`
	Story           string `json:"story"`
	ExtraTestArgs   string `json:"extra_test_args"`
	Repository      string `json:"repository"`
	BugId           string `json:"bug_id"`
	User            string `json:"user"`
}

type BisectJobCreateRequest struct {
	ComparisonMode      string `json:"comparison_mode"`
	StartGitHash        string `json:"start_git_hash"`
	EndGitHash          string `json:"end_git_hash"`
	Configuration       string `json:"configuration"`
	Benchmark           string `json:"benchmark"`
	Story               string `json:"story"`
	Chart               string `json:"chart"`
	Statistic           string `json:"statistic"`
	ComparisonMagnitude string `json:"comparison_magnitude"`
	Pin                 string `json:"pin"`
	Project             string `json:"project"`
	BugId               string `json:"bug_id"`
	User                string `json:"user"`
	AlertIDs            string `json:"alert_ids"`
	TestPath            string `json:"test_path"`
}

type CreatePinpointResponse struct {
	JobID  string `json:"jobId"`
	JobURL string `json:"jobUrl"`
}

type FetchJobStateRequest struct {
	JobID string `json:"job_id"`
}

type Commit struct {
	Repository     string  `json:"repository"`
	GitHash        string  `json:"git_hash"`
	URL            string  `json:"url,omitempty"`
	Author         string  `json:"author,omitempty"`
	Created        string  `json:"created,omitempty"`
	Subject        string  `json:"subject,omitempty"`
	Message        string  `json:"message,omitempty"`
	CommitBranch   *string `json:"commit_branch,omitempty"`
	CommitPosition *int    `json:"commit_position,omitempty"`
	ReviewURL      *string `json:"review_url,omitempty"`
	ChangeID       *string `json:"change_id,omitempty"`
}

type Patch struct {
	Server   string `json:"server"`
	Change   string `json:"change"`
	Revision string `json:"revision"`
	URL      string `json:"url,omitempty"`
	Author   string `json:"author,omitempty"`
	Created  string `json:"created,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Message  string `json:"message,omitempty"`
}

type Change struct {
	Commits []Commit    `json:"commits"`
	Patch   *Patch      `json:"patch,omitempty"`
	Args    interface{} `json:"args,omitempty"` // Can be a string or an array of strings.
	Label   string      `json:"label,omitempty"`
	Variant *int        `json:"variant,omitempty"`
}

type ExecutionDetail struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	URL   string `json:"url,omitempty"`
}

type Exception struct {
	Message   string `json:"message"`
	Traceback string `json:"traceback"`
}

type Execution struct {
	Completed bool              `json:"completed"`
	Exception *Exception        `json:"exception,omitempty"`
	Details   []ExecutionDetail `json:"details"`
}

type Attempt struct {
	Executions []Execution `json:"executions"`
}

type StateItem struct {
	Change       Change            `json:"change"`
	Comparisons  map[string]string `json:"comparisons,omitempty"`
	ResultValues []float64         `json:"result_values,omitempty"`
	Attempts     []Attempt         `json:"attempts,omitempty"`
}

type FetchJobStateResponse struct {
	JobID                string            `json:"job_id"`
	Configuration        string            `json:"configuration"`
	ResultsURL           *string           `json:"results_url,omitempty"`
	ImprovementDirection *int              `json:"improvement_direction,omitempty"`
	Arguments            map[string]string `json:"arguments"`
	BugID                *int              `json:"bug_id,omitempty"`
	Project              *string           `json:"project,omitempty"`
	ComparisonMode       string            `json:"comparison_mode"`
	Name                 string            `json:"name"`
	User                 *string           `json:"user,omitempty"`
	Created              string            `json:"created"`
	Updated              string            `json:"updated"`
	StartedTime          string            `json:"started_time"`
	DifferenceCount      *int              `json:"difference_count,omitempty"`
	Exception            *Exception        `json:"exception,omitempty"`
	Status               string            `json:"status"`
	CancelReason         *string           `json:"cancel_reason,omitempty"`
	BatchID              *string           `json:"batch_id,omitempty"`
	Bots                 []string          `json:"bots"`

	// Fields from OPTION_STATE
	Metric string      `json:"metric"`
	Quests []string    `json:"quests"`
	State  []StateItem `json:"state"`
}

type LegacyJobSummary struct {
	JobID          string            `json:"job_id"`
	Name           string            `json:"name"`
	Benchmark      string            `json:"benchmark"`
	Configuration  string            `json:"configuration"`
	Story          string            `json:"story"`
	User           string            `json:"user"`
	Created        string            `json:"created"`
	Status         string            `json:"status"`
	ComparisonMode string            `json:"comparison_mode"`
	Arguments      map[string]string `json:"arguments"`
}

type LegacyQueryJobListResponse struct {
	Jobs       []LegacyJobSummary `json:"jobs"`
	PrevCursor string             `json:"prev_cursor"`
	NextCursor string             `json:"next_cursor"`
}
