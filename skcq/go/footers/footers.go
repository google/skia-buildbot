package footers

import (
	"strings"

	"go.skia.org/infra/go/skerr"
)

const (

	// If this is false and the full CQ is triggered then CQ will fail.
	CommitFooter string = "Commit"

	// Does not cancel try jobs if new CODE_CHANGE patchsets are uploaded.
	DoNotCancelTryjobsFooter string = "Cq-Do-Not-Cancel-Tryjobs"

	// Includes the specified tryjobs. The tryjobs will be in this format:
	// bucket1:bot1,bot2;bucket2:bot3,bot4
	IncludeTryjobsFooter string = "Cq-Include-Trybots"

	// Skips the tree status check if this is true.
	NoTreeChecksFooter string = "No-Tree-Checks"

	// Triggering and checking for tryjobs will be skipped if this is true.
	NoTryFooter string = "No-Try"

	// If true and the change has other open changes that will be submitted at the
	// same time then the CQ will return failure. Not applicable for dry-runs.
	NoDependencyChecksFooter string = "No-Dependency-Checks"

	// If true then all successful try jobs are rerun regardless of who triggered
	// them.
	RerunTryjobsFooter string = "Rerun-Tryjobs"
)

// ParseIncludeTryJobsFooter parses the IncludeTryjobsFooter and returns
// a map of buckets to bots.
// The string is expected to be in this format:
// "bucket1:bot1,bot2;bucket2:bot3"
// For the above string the following will be returned:
// {"bucket1": ["bot1", "bot2"], "bucket2": ["bot3"]}
func ParseIncludeTryjobsFooter(footer string) (map[string][]string, error) {
	bucketsToTryjobs := map[string][]string{}
	// Create array of ["bucket1:bot1,bot2", "bucket2:bot3,bot4"]
	bucketsWithBots := strings.Split(footer, ";")
	for _, bb := range bucketsWithBots {
		tokens := strings.Split(bb, ":")
		if len(tokens) != 2 {
			return nil, skerr.Fmt("Invalid format of \"%s: %s\"", IncludeTryjobsFooter, footer)
		}
		bucket := tokens[0]
		bots := strings.Split(tokens[1], ",")
		if existingBots, ok := bucketsToTryjobs[bucket]; ok {
			bucketsToTryjobs[bucket] = append(existingBots, bots...)
		} else {
			bucketsToTryjobs[bucket] = bots
		}
	}
	return bucketsToTryjobs, nil
}
