package footers

import (
	"strconv"
	"strings"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

type CQSupportedFooter string

const (

	// If this is false and the full CQ is triggered then CQ will fail.
	CommitFooter CQSupportedFooter = "Commit"

	// Does not cancel try jobs if new CODE_CHANGE patchsets are uploaded.
	DoNotCancelTryjobsFooter CQSupportedFooter = "Cq-Do-Not-Cancel-Tryjobs"

	// Includes the specified tryjobs. The tryjobs will be in this format:
	// bucket1:bot1,bot2;bucket2:bot3,bot4
	IncludeTryjobsFooter CQSupportedFooter = "Cq-Include-Trybots"

	// Skips the tree status check if this is true.
	NoTreeChecksFooter CQSupportedFooter = "No-Tree-Checks"

	// Triggering and checking for tryjobs will be skipped if this is true.
	NoTryFooter CQSupportedFooter = "No-Try"

	// If true and the change has other open changes that will be submitted at the
	// same time then the CQ will return failure. Not applicable for dry-runs.
	NoDependencyChecksFooter CQSupportedFooter = "No-Dependency-Checks"

	// If true then all successful try jobs are rerun regardless of who triggered
	// them.
	RerunTryjobsFooter CQSupportedFooter = "Rerun-Tryjobs"
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

// GetFootersMap parses the specified commit msg and returns it's footers.
func GetFootersMap(commitMsg string) map[string]string {
	footersMap := map[string]string{}
	_, footers := git.SplitTrailers(commitMsg)
	for _, f := range footers {
		rs := git.TrailerRegex.FindStringSubmatch(f)
		if len(rs) != 3 {
			sklog.Errorf("Could not parse footer %s from the commitMsg %s", f, commitMsg)
			continue
		}
		footersMap[rs[1]] = rs[2]
	}

	return footersMap
}

// GetBoolVal looks for the specified footer in the footersMap and returns
// it's boolean value. If the footer is not found then false is returned.
// If the value is not boolean then false is returned and an error is logged.
func GetBoolVal(footersMap map[string]string, supportedFooter CQSupportedFooter, issue int64) bool {
	if val, ok := footersMap[string(supportedFooter)]; ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			sklog.Errorf("Could not parse bool value out of \"%s: %s\" in %d", supportedFooter, val, issue)
			return false
		} else {
			if b {
				return b
			}
		}
	}
	return false
}

// GetStringVal looks for the specified footer in the footersMap and returns
// it's strings value. If the footer is not found then an empty string is
// returned.
func GetStringVal(footersMap map[string]string, supportedFooter CQSupportedFooter) string {
	if val, ok := footersMap[string(supportedFooter)]; ok {
		return val
	}
	return ""
}
