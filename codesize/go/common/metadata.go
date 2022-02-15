// This package contains data structures shared between the CodeSize web server and the
// CodeSize-* tasks in the Skia repository.
package common

// BloatyOutputMetadata contains metadata about a Bloaty output file. It contains the Bloaty version
// and command-line arguments used, and information about the CI task where Bloaty was invoked.
//
// This struct is serialized into a JSON file to be uploaded to GCS alongside a Bloaty output file.
// All Bloaty output files should be accompanied by a JSON file with metadata.
type BloatyOutputMetadata struct {
	Version   int    `json:"version"` // Schema version of this file, starting at 1.
	Timestamp string `json:"timestamp"`

	SwarmingTaskID string `json:"swarming_task_id"`
	SwarmingServer string `json:"swarming_server"`

	TaskID          string `json:"task_id"`
	TaskName        string `json:"task_name"`
	CompileTaskName string `json:"compile_task_name"`
	BinaryName      string `json:"binary_name"`

	BloatyCipdVersion string   `json:"bloaty_cipd_version"`
	BloatyArgs        []string `json:"bloaty_args"`

	PatchIssue  string `json:"patch_issue"`
	PatchServer string `json:"patch_server"`
	PatchSet    string `json:"patch_set"`
	Repo        string `json:"repo"`
	Revision    string `json:"revision"`

	CommitTimestamp string `json:"commit_timestamp"`
	Author          string `json:"author"`
	Subject         string `json:"subject"`
}
