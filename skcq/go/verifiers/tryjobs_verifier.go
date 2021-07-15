package verifiers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	// Time to wait before re-running old jobs for fresh results.
	TryJobStaleTimeoutSecs = 24 * 60 * 60

	BuildBucketDefaultSkiaProject = "skia"

	BuildBucketDefaultSkiaBucket  = "skia.primary"
	BuildBucketInternalSkiaBucket = "skia.internal"
	BuildBucketStagingSkiaBucket  = "skia.testing"

	CancelBuildsMsg = "SkCQ is cleaning up try jobs from older patchsets"
)

// timeNowFunc allows tests to mock out time.Now() for testing.
var timeNowFunc = time.Now

// NewTryJobsVerifier returns an instance of TryJobsVerifier.
func NewTryJobsVerifier(httpClient *http.Client, cr codereview.CodeReview, tasksCfg *specs.TasksCfg, footersMap map[string]string, visibilityType config.VisibilityType) (types.Verifier, error) {
	// Find gerritURL (eg: skia-review.googlesource.com).
	issueURL := cr.Url(0)
	u, err := url.Parse(issueURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not url.Parse %s", issueURL)
	}

	return &TryJobsVerifier{
		bb2:            buildbucket.NewClient(httpClient),
		cr:             cr,
		gerritURL:      u.Host,
		tasksCfg:       tasksCfg,
		footersMap:     footersMap,
		visibilityType: visibilityType,
	}, nil
}

// TryJobsVerifier implements the types.Verifier interface.
type TryJobsVerifier struct {
	bb2            buildbucket.BuildBucketInterface
	cr             codereview.CodeReview
	tasksCfg       *specs.TasksCfg
	gerritURL      string
	footersMap     map[string]string
	visibilityType config.VisibilityType
}

// Name implements the types.Verifier interface.
func (tv *TryJobsVerifier) Name() string {
	return "TryJobsVerifier"
}

// Verify implements the types.Verifier interface.
func (tv *TryJobsVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {
	// If CQ tryjobs list is empty then return success. No bots to run.
	if tv.tasksCfg == nil || tv.tasksCfg.CommitQueue == nil || len(tv.tasksCfg.CommitQueue) == 0 {
		return types.VerifierSuccessState, "This repo+branch has no CQ try jobs", nil
	}

	// If "No-Try: true" has been specified then immediately return success.
	noTry := git.GetBoolFooterVal(tv.footersMap, footers.NoTryFooter, ci.Issue)
	if noTry {
		return types.VerifierSuccessState, fmt.Sprintf("Try jobs check is skipped because \"%s: %t\" has been specified", footers.NoTryFooter, noTry), nil
	}

	// Figure out which BB bucket should be used for this change.
	var bbBucket string
	switch tv.visibilityType {
	case config.InternalVisibility:
		bbBucket = BuildBucketInternalSkiaBucket
	case config.StagingVisibility:
		bbBucket = BuildBucketStagingSkiaBucket
	default:
		bbBucket = BuildBucketDefaultSkiaBucket
	}

	// Search for all try jobs that have been triggered on all equivalent
	// patchsets. We do this because if PS5 and PS4 are NO_CHANGE patchsets
	// and PS3 is a CODE_CHANGE patchset, then we want to consider the try jobs
	// on PS5 + PS4 + PS3.
	latestPatchSetID := tv.cr.GetLatestPatchSetID(ci)
	equivalentPatchSetIDS := tv.cr.GetEquivalentPatchSetIDs(ci, latestPatchSetID)
	tryJobsOnChange := []*buildbucketpb.Build{}
	for _, p := range equivalentPatchSetIDS {
		tryJobs, err := tv.bb2.GetTrybotsForCL(ctx, ci.Issue, p, "https://"+tv.gerritURL, nil)
		if err != nil {
			return "", "", skerr.Wrapf(err, "Could not get tryjobs for %d", ci.Issue)
		}
		tryJobsOnChange = append(tryJobsOnChange, tryJobs...)
	}
	// Create map of builder name to buildbucketpb.Build to easily reference
	// which builds are already on the change.
	nameToBuilderOnChange := map[string]*buildbucketpb.Build{}
	for _, b := range tryJobsOnChange {
		if existingTryJob, ok := nameToBuilderOnChange[b.GetBuilder().GetBuilder()]; ok {
			// If existing try job is older then replace it.
			if existingTryJob.GetCreateTime().Seconds < b.GetCreateTime().Seconds {
				nameToBuilderOnChange[b.GetBuilder().GetBuilder()] = b
			}
		} else {
			nameToBuilderOnChange[b.GetBuilder().GetBuilder()] = b
		}
	}

	// Get all CQ try jobs defined in tasksCfg for this change.
	cqTryjobsToConfigs := tv.tasksCfg.CommitQueue

	// Check, parse, and add the try jobs in IncludeTryjobsFooter if specified.
	includeTryJobs, err := tv.getIncludeFooterTryJobs(ci.Issue, bbBucket)
	if err != nil {
		return types.VerifierFailureState, err.Error(), nil
	}
	for _, t := range includeTryJobs {
		// Only add to the cqTryjobsToConfigs map if it is not already there.
		if _, ok := cqTryjobsToConfigs[t]; !ok {
			sklog.Infof("[%d] Added tryjob %s because it was specified in %s", ci.Issue, t, footers.IncludeTryjobsFooter)
			// Add a default CommitQueueJobConfig config for try jobs listed in
			// IncludeTryjobsFooter.
			cqTryjobsToConfigs[t] = &specs.CommitQueueJobConfig{}
		}
	}

	// See if successful try jobs should be retriggered.
	rerunTryJobs := git.GetBoolFooterVal(tv.footersMap, footers.RerunTryjobsFooter, ci.Issue)
	if rerunTryJobs {
		sklog.Infof("[%d] \"%s: %t\" has been specified. All successful try jobs that completed before this cq attempt will not be reused.", ci.Issue, footers.RerunTryjobsFooter, rerunTryJobs)
	}

	// Loop through the CQ try jobs and populate these slices.
	extraInfoForUIMsgs := []string{}
	botsToExperimental := map[string]bool{}
	skippedTryJobs := []string{}
	staleTryJobs := []string{}
	reuseSuccessTryJobs := []string{}
	reuseRunningTryJobs := []string{}
	notFoundTryJobs := []string{}
	for cqJobName, cqCfg := range cqTryjobsToConfigs {
		// Store the experimental status of this try job.
		botsToExperimental[cqJobName] = cqCfg.Experimental

		// Make sure the location regex (if specified) matches before we consider this job.
		if len(cqCfg.LocationRegexes) > 0 {
			matched, locationRegexMatch, err := tv.doesLocationRegexMatch(ctx, ci, latestPatchSetID, cqCfg.LocationRegexes)
			if err != nil {
				return "", "", skerr.Wrap(err)
			}
			if matched {
				// Process this try job.
				addedReason := fmt.Sprintf("%s added because it matched the location regex: %s", cqJobName, locationRegexMatch)
				sklog.Infof("[%d] %s", ci.Issue, addedReason)
				extraInfoForUIMsgs = append(extraInfoForUIMsgs, addedReason)
			} else {
				// Ignore this CQ job.
				skippedReason := fmt.Sprintf("%s skipped because it did not match any of the location regexes: %s", cqJobName, strings.Join(cqCfg.LocationRegexes, ","))
				sklog.Infof("[%d] %s", ci.Issue, skippedReason)
				extraInfoForUIMsgs = append(extraInfoForUIMsgs, skippedReason)
				skippedTryJobs = append(skippedTryJobs, cqJobName)
				continue
			}
		}

		// Check to see if this try job already exists on the current change.
		if build, ok := nameToBuilderOnChange[cqJobName]; ok {

			if timeNowFunc().Unix()-build.GetCreateTime().GetSeconds() >= TryJobStaleTimeoutSecs {
				// If a job is stale then it needs to be retriggered regardless of it's state.
				staleTryJobs = append(staleTryJobs, cqJobName)

			} else if build.GetStatus() == buildbucketpb.Status_SUCCESS {
				if rerunTryJobs && build.GetEndTime().GetSeconds() < startTime {
					// Do not consider these successful jobs if rerunTryJobs is true.
					notFoundTryJobs = append(notFoundTryJobs, cqJobName)
				} else {
					// If a job is successful then reuse it regardless of if a user triggered it or the CQ triggered it.
					reuseSuccessTryJobs = append(reuseSuccessTryJobs, cqJobName)
				}

			} else if build.GetStatus() == buildbucketpb.Status_STARTED || build.GetStatus() == buildbucketpb.Status_SCHEDULED {
				if exp, ok := botsToExperimental[cqJobName]; ok && exp {
					sklog.Infof("[%d] The experimental bot %s is still running. Going to consider it successful", ci.Issue, cqJobName)
					reuseSuccessTryJobs = append(reuseSuccessTryJobs, cqJobName)
				} else {
					// If a job is running then consider it part of the current attempt regardless of who triggered it.
					reuseRunningTryJobs = append(reuseRunningTryJobs, cqJobName)
				}

			} else if build.GetStatus() == buildbucketpb.Status_CANCELED || build.GetStatus() == buildbucketpb.Status_FAILURE || build.GetStatus() == buildbucketpb.Status_INFRA_FAILURE {
				if build.GetEndTime().GetSeconds() < startTime {
					// If a job failed before the current cq attempt then it needs to be retriggered because it was
					// not part of the current CQ attempt.
					sklog.Infof("[%d] %s failed before the current CQ attempt of %d. Ignoring it and it will be retriggered.", ci.Issue, cqJobName, startTime)
					notFoundTryJobs = append(notFoundTryJobs, cqJobName)
				} else if exp, ok := botsToExperimental[cqJobName]; ok && exp {
					// This is an experimental bot. Consider it successful.
					sklog.Infof("[%d] The experimental bot %s failed. Going to consider it successful", ci.Issue, cqJobName)
					reuseSuccessTryJobs = append(reuseSuccessTryJobs, cqJobName)
				} else {
					// If a job failed after the current cq attempt started then the verifier has failed.
					sklog.Infof("[%d] The try job %s has failed", ci.Issue, cqJobName)
					return types.VerifierFailureState, fmt.Sprintf("%s has failed", cqJobName), nil
				}

			} else {
				// Not sure what state this is in. Log an error.
				sklog.Errorf("[%d] Unknown state %s for try job %s", ci.Issue, build.GetStatus(), cqJobName)
				// Returning an error for now.
				return "", "", skerr.Fmt("Unknown state %s for try job %s", build.GetStatus(), cqJobName)

			}
		} else {
			// The try job has not been triggered on the change yet.
			notFoundTryJobs = append(notFoundTryJobs, cqJobName)
		}
	}

	sklog.Infof("[%d] For CQ try jobs- Skipped %d try jobs. Found %d stale try jobs. %d successful reusable try jobs. %d running reusable try jobs. %d try jobs were not found. %d Total CQ try jobs", ci.Issue, len(skippedTryJobs), len(staleTryJobs), len(reuseSuccessTryJobs), len(reuseRunningTryJobs), len(notFoundTryJobs), len(tv.tasksCfg.CommitQueue))

	// Trigger all stale and not found try jobs
	triggerTryJobs := append(staleTryJobs, notFoundTryJobs...)
	if len(triggerTryJobs) > 0 {
		sklog.Infof("[%d] Triggering %d try jobs", ci.Issue, len(triggerTryJobs))
		botsToTags := map[string]map[string]string{}
		for _, t := range triggerTryJobs {
			tags := map[string]string{
				"triggered_by": "skcq",
			}
			if experimental, ok := botsToExperimental[t]; ok {
				tags["cq_experimental"] = strconv.FormatBool(experimental)
			}
			botsToTags[t] = tags
		}
		respBuilds, err := tv.bb2.ScheduleBuilds(ctx, triggerTryJobs, botsToTags, ci.Issue, latestPatchSetID, tv.gerritURL, ci.Project, BuildBucketDefaultSkiaProject, bbBucket)
		if err != nil {
			return "", "", skerr.Wrapf(err, "Could not trigger %+v tryjobs for %d", triggerTryJobs, ci.Issue)
		}
		// Make sure the try jobs were succesfully triggered. This step should not be necessary but if we
		// specify a repo/bucket that does not exist the ScheduleBuilds silently succeeds.
		newTryJobsOnChange, err := tv.bb2.GetTrybotsForCL(ctx, ci.Issue, latestPatchSetID, "https://"+tv.gerritURL, map[string]string(nil))
		if err != nil {
			return "", "", skerr.Wrapf(err, "Could not get tryjobs for %d", ci.Issue)
		}
		for _, b := range respBuilds {
			found := false
			// Make sure this build is in the new tryjobs on change.
			for _, n := range newTryJobsOnChange {
				if b.GetId() == n.GetId() {
					found = true
				}
			}
			if !found {
				return "", "", skerr.Fmt("[%d] %s with id %d was scheduled but did not show up on buildbucket", ci.Issue, b.GetBuilder().GetBuilder(), b.GetId())
			}
		}

	}

	extraInfoForUIMsg := ""
	if len(extraInfoForUIMsgs) > 0 {
		extraInfoForUIMsg = fmt.Sprintf("\n%s", strings.Join(extraInfoForUIMsgs, "\n"))
	}
	waitingTryJobs := append(reuseRunningTryJobs, notFoundTryJobs...)
	if len(waitingTryJobs) > 0 {
		return types.VerifierWaitingState, fmt.Sprintf("Waiting for %d try jobs to complete.%s", len(waitingTryJobs), extraInfoForUIMsg), nil
	} else {
		if len(reuseSuccessTryJobs) != len(tv.tasksCfg.CommitQueue)-len(skippedTryJobs) {
			// This *should* not happen.
			return "", "", skerr.Fmt("[%d] %d successful try jobs does not match the %d total try jobs - %d skipped try jobs", ci.Issue, len(reuseSuccessTryJobs), len(tv.tasksCfg.CommitQueue), len(skippedTryJobs))
		}
		// If we are not waiting on anything and there were no failures then they were all successful.
		return types.VerifierSuccessState, fmt.Sprintf("CQ Try jobs were successful.%s", extraInfoForUIMsg), nil
	}
}

// Cleanup implements the types.Verifier interface.
func (tv *TryJobsVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	// If "Cq-Do-Not-Cancel-Tryjobs: true" has been specified then immediately return success.
	noCancelTryJobs := git.GetBoolFooterVal(tv.footersMap, footers.DoNotCancelTryjobsFooter, ci.Issue)
	if noCancelTryJobs {
		sklog.Infof("Not checking for and not cancelling try jobs for %d/%d because %s id specified in footers", ci.Issue, cleanupPatchsetID, footers.DoNotCancelTryjobsFooter)
		return
	}

	//Refresh the change to get the latest patchset ID.
	refreshedChange, err := tv.cr.GetIssueProperties(ctx, ci.Issue)
	if err != nil {
		sklog.Errorf("Could not get refreshed change for %d in cleanup of %s", ci.Issue, tv.Name())
		return
	}
	refreshedPSID := tv.cr.GetEarliestEquivalentPatchSetID(refreshedChange)

	if cleanupPatchsetID != refreshedPSID {
		// Find all the builds triggered by CQ and then cancel them.
		builds, err := tv.bb2.GetTrybotsForCL(ctx, refreshedChange.Issue, cleanupPatchsetID, "https://"+tv.gerritURL, map[string]string{"triggered_by": "skcq"})
		if err != nil {
			sklog.Errorf("Could not search for trybots for CL %d in cleanup of %s: %s", ci.Issue, tv.Name(), err)
			return
		}
		for _, b := range builds {
			buildIDsToCancel := []int64{}
			if b.GetStatus() == buildbucketpb.Status_STARTED || b.GetStatus() == buildbucketpb.Status_SCHEDULED {
				sklog.Infof("[%d] old patchset %d has a still running try jobs: %s. It will be canceled.", refreshedChange.Issue, cleanupPatchsetID, b.GetBuilder().Builder)
				buildIDsToCancel = append(buildIDsToCancel, b.GetId())
			}
			if len(buildIDsToCancel) > 0 {
				if _, err := tv.bb2.CancelBuilds(ctx, buildIDsToCancel, CancelBuildsMsg); err != nil {
					sklog.Errorf("Could not cleanup buildbucket builds of IDs %+v: %s", buildIDsToCancel, err)
					return
				}
			}
		}
	}

	return
}

// getIncludeFooterTryJobs parses footers for the footers.IncludeTryjobsFooter
// and returns try jobs from it. If the specified project or bucket does not
// match it is expected, then an error is returned.
func (tv *TryJobsVerifier) getIncludeFooterTryJobs(issue int64, bbBucket string) ([]string, error) {
	// Check, parse, and get the try jobs in IncludeTryjobsFooter if specified.
	includeTryjobsFooter := git.GetStringFooterVal(tv.footersMap, footers.IncludeTryjobsFooter)
	if includeTryjobsFooter == "" {
		return []string{}, nil
	}
	includeTryJobsMap, err := footers.ParseIncludeTryjobsFooter(includeTryjobsFooter)
	if err != nil {
		sklog.Errorf("[%d] Could not parse %s: %s", issue, includeTryjobsFooter, err)
		return []string{}, nil
	}
	retTryJobs := []string{}
	for bucket, tryJobs := range includeTryJobsMap {
		projectAndBucket := strings.Split(bucket, "/")
		var p, b string
		if len(projectAndBucket) == 2 {
			// This format is supported. eg: "skia/skia.primary".
			p = projectAndBucket[0]
			b = projectAndBucket[1]
		} else {
			// Another supported format is "luci.skia.skia.primary" try that one out now.
			projectAndBucket = strings.Split(bucket, ".")
			if len(projectAndBucket) != 4 {
				return nil, skerr.Fmt("Unsupported bucket value of \"%s\" in %+v", bucket, includeTryJobsMap)
			}
			p = projectAndBucket[1]
			b = fmt.Sprintf("%s.%s", projectAndBucket[2], projectAndBucket[3])
		}
		if p != BuildBucketDefaultSkiaProject {
			return nil, skerr.Fmt("Could not recognize bb project \"%s\" in %+v", p, includeTryJobsMap)
		}
		if b != bbBucket {
			return nil, skerr.Fmt("Specified bucket \"%s\" is different than expected bucket %s in %+v", b, bbBucket, includeTryJobsMap)
		}
		retTryJobs = append(retTryJobs, tryJobs...)
	}
	return retTryJobs, nil
}

// doesLocationRegexMatch looks at if the file patches modify by the change
// match any of the provided location regexes. If any match then the matching
// regex is returned (for logging purposes).
func (tv *TryJobsVerifier) doesLocationRegexMatch(ctx context.Context, ci *gerrit.ChangeInfo, patchsetID int64, locationRegexes []string) (bool, string, error) {
	changedFiles, err := tv.cr.GetFileNames(ctx, ci)
	if err != nil {
		return false, "", skerr.Wrapf(err, "Could not get file names from %d/%d", ci.Issue, patchsetID)
	}
	for _, locationRegex := range locationRegexes {
		r, err := regexp.Compile(locationRegex)
		if err != nil {
			return false, "", skerr.Wrapf(err, "%s location regex does not compile", locationRegex)
		}
		// Run regex on all changed files.
		for _, cf := range changedFiles {
			if r.MatchString(cf) {
				return true, locationRegex, nil
			}
		}
	}
	return false, "", nil
}
