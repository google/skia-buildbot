package footers

import (
	"strconv"

	"go.skia.org/infra/go/git"
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
	// TODO(rmistry): Trigger bots in luci.chromium.try:linux-blink-rel ?
	IncludeTryjobsFooter CQSupportedFooter = "Cq-Include-Trybots"

	// Skips the tree status check if this is true.
	NoTreeChecksFooter CQSupportedFooter = "No-Tree-Checks"

	// // Do not run the presubmit check if this is true.
	// NoPresubmitFooter CQSupportedFooter = "No-Presubmit"

	// Triggering and checking for tryjobs will be skipped if this is true.
	NoTryFooter CQSupportedFooter = "No-Try"

	// If true and the change has other open changes that will be submitted at the
	// same time then the CQ will return failure. Not applicable for dry-runs.
	NoDependencyChecksFooter CQSupportedFooter = "No-Dependency-Checks"

	// If true then all CQ verifiers are rerun. This is useful if you want try jobs
	// newer than 24 hours to be retriggered.
	// UPDATE THE DOC
	RerunTryjobsFooter CQSupportedFooter = "Rerun-Tryjobs"
)

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

// Logs an error if the val is not bool.
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
