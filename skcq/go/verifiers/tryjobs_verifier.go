package verifiers

// Notes:
// Will have to add ability to trigger builds in go/buildbucket
// Also look at task_scheduler/go/tryjobs
// and buildbucket_util.py in cq
// and verification/tryjob.py in cq

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
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/task_scheduler/go/specs"
)

const (
	// Time to wait before re-running old jobs for fresh results.
	// TryJobStaleTimeoutSecs = 24 * 60 * 60
	TryJobStaleTimeoutSecs = 15 * 60

	BuildBucketDefaultSkiaProject = "skia"
	BuildBucketDefaultSkiaBucket  = "skia.primary"
)

var (
	// It will be difficult to keep this synchronized with
	// https://skia.googlesource.com/skia/+/infra/config/main.star
	stagingReposToBuckets = map[string]string{
		"skiabot-test": "skia.testing",
	}
)

func NewTryJobsVerifier(httpClient *http.Client, cr codereview.CodeReview, tasksCfg *specs.TasksCfg) (Verifier, error) {

	return &TryJobsVerifier{
		bb2:      buildbucket.NewClient(httpClient),
		cr:       cr,
		tasksCfg: tasksCfg,
	}, nil
}

type TryJobsVerifier struct {
	// Maybe will not need this??
	// bb: *buildbucket_Api.Service

	bb2      buildbucket.BuildBucketInterface
	cr       codereview.CodeReview
	tasksCfg *specs.TasksCfg
}

func (tv *TryJobsVerifier) Name() string {
	return "[TryJobsVerifier]"
}

// Need to do the 24 hour thing as well. Unless it has a disable_reuse. What is that?
// Look at cq/verifications/tryjob_utils.py
func (tv *TryJobsVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state VerifierState, reason string, err error) {

	// If CQ tryjobs list is empty then return success. No bots to run.
	if tv.tasksCfg == nil || tv.tasksCfg.CommitQueue == nil || len(tv.tasksCfg.CommitQueue) == 0 {
		return SuccessState, fmt.Sprintf("%s This repo+branch has no CQ try jobs", tv.Name()), nil
	}

	// Create map of builder name to buildbucketpb.Build.
	nameToBuilderOnChange := map[string]*buildbucketpb.Build{}
	issueURL := tv.cr.Url(0)
	u, err := url.Parse(issueURL)
	if err != nil {
		return "", "", skerr.Fmt("Could not url.Parse %s: %s", issueURL, err)
	}

	// Search for all trybots that have been triggered on all equivalent patchsets.
	gerritURL := u.Host
	latestPatchSetID := tv.cr.GetLatestPatchSetID(ci)
	equivalentPatchSetIDS := tv.cr.GetEquivalentPatchSetIDs(ci, latestPatchSetID)
	tryJobsOnChange := []*buildbucketpb.Build{}
	for _, p := range equivalentPatchSetIDS {
		tryJobs, err := tv.bb2.GetTrybotsForCL(ctx, ci.Issue, p, issueURL)
		if err != nil {
			return "", "", skerr.Fmt("Could not get tryjobs for %d: %s", ci.Issue, err)
		}
		tryJobsOnChange = append(tryJobsOnChange, tryJobs...)
	}
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

	// Create map of try job name to experimental stats
	botsToExperimental := map[string]string{}
	// Cross reference try jobs on the change with CQ tryjobs.
	skippedTryJobs := []string{}
	staleTryJobs := []string{}
	reuseSuccessTryJobs := []string{}
	reuseRunningTryJobs := []string{}
	notFoundTryJobs := []string{}
	for cqJobName, cqCfg := range tv.tasksCfg.CommitQueue {
		botsToExperimental[cqJobName] = strconv.FormatBool(cqCfg.Experimental)
		// Make sure the location regex (if specified) matches before we consider this job.
		if len(cqCfg.LocationRegexes) > 0 {
			changedFiles, err := tv.cr.GetFileNames(ctx, ci)
			if err != nil {
				return "", "", skerr.Fmt("Could not get file names from %d/%d: %s", ci.Issue, latestPatchSetID, err)
			}
			locationRegexMatch := false
		regexesLoop:
			for _, locationRegex := range cqCfg.LocationRegexes {
				r, err := regexp.Compile(locationRegex)
				if err != nil {
					return "", "", skerr.Fmt("%s location regex does not compile: %s", locationRegex, err)
				}
				// Run regex on all changed files.
				for _, cf := range changedFiles {
					if r.MatchString(cf) {
						locationRegexMatch = true
						sklog.Infof("[%d] Adding CQ job %s because it did not matches the location regex: %s", ci.Issue, locationRegexMatch)
						break regexesLoop
					}
				}
			}
			if !locationRegexMatch {
				// Ignore this CQ job.
				sklog.Infof("[%d] Skipping CQ job %s because it did not match any of the location regexes", ci.Issue, cqJobName)
				skippedTryJobs = append(skippedTryJobs, cqJobName)
				continue
			}
		}

		// Has the try job already been triggered on the change?
		if build, ok := nameToBuilderOnChange[cqJobName]; ok {

			if time.Now().Unix()-build.GetCreateTime().GetSeconds() >= TryJobStaleTimeoutSecs {
				// If a job is stale then it needs to be retriggered regardless of it's state.
				staleTryJobs = append(staleTryJobs, cqJobName)

			} else if build.GetStatus() == buildbucketpb.Status_SUCCESS {
				// If a job is successful then reuse it regardless of if a user triggered it or the CQ triggered it.
				reuseSuccessTryJobs = append(reuseSuccessTryJobs, cqJobName)

			} else if build.GetStatus() == buildbucketpb.Status_STARTED || build.GetStatus() == buildbucketpb.Status_SCHEDULED {
				// If a job is running then consider it part of the current attempt regardless of who triggered it.
				reuseRunningTryJobs = append(reuseRunningTryJobs, cqJobName)

			} else if build.GetStatus() == buildbucketpb.Status_CANCELED || build.GetStatus() == buildbucketpb.Status_FAILURE || build.GetStatus() == buildbucketpb.Status_INFRA_FAILURE {
				if build.GetEndTime().GetSeconds() < startTime {
					// If a job failed before the current cq attempt then it needs to be retriggered because it was
					// not part of the current CQ attempt.
					sklog.Infof("[%d] %s failed before the current CQ attempt of %d. Ignoring it and it will be retriggered.", ci.Issue, cqJobName, startTime)
					notFoundTryJobs = append(notFoundTryJobs, cqJobName)
				} else {
					// If a job failed after the current cq attempt started then the verifier has failed.
					sklog.Infof("[%d] The try job %s has failed", ci.Issue, cqJobName)
					return FailureState, fmt.Sprintf("%s %s has failed", tv.Name(), cqJobName), nil
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

	// Trigger all try jobs.
	bbBucket, ok := stagingReposToBuckets[ci.Project]
	if !ok {
		bbBucket = BuildBucketDefaultSkiaBucket
	}
	triggerTryJobs := append(staleTryJobs, notFoundTryJobs...)
	if len(triggerTryJobs) > 0 {
		sklog.Infof("[%d] Triggering %d try jobs: %s", ci.Issue, len(triggerTryJobs), strings.Join(triggerTryJobs, ","))

		respBuilds, err := tv.bb2.ScheduleBuilds(ctx, triggerTryJobs, botsToExperimental, ci.Issue, latestPatchSetID, gerritURL, ci.Project, BuildBucketDefaultSkiaProject, bbBucket)
		if err != nil {
			return "", "", skerr.Fmt("Could not trigger %+v tryjobs for %d: %s", triggerTryJobs, ci.Issue, err)
		}
		// Make sure the try jobs were succesfully triggered. This step should not be necessary but if we
		// specify a repo/bucket that does not exist the ScheduleBuilds silently succeeds.
		newTryJobsOnChange, err := tv.bb2.GetTrybotsForCL(ctx, ci.Issue, latestPatchSetID, issueURL)
		if err != nil {
			return "", "", skerr.Fmt("Could not get tryjobs for %d: %s", ci.Issue, err)
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

	waitingTryJobs := append(reuseRunningTryJobs, notFoundTryJobs...)
	if len(waitingTryJobs) > 0 {
		return WaitingState, fmt.Sprintf("%s Waiting for: %s", tv.Name(), strings.Join(waitingTryJobs, ",")), nil
	} else {
		if len(reuseSuccessTryJobs) != len(tv.tasksCfg.CommitQueue)-len(skippedTryJobs) {
			// This *should* not happen.
			return "", "", skerr.Fmt("[%d] %d successful try jobs does not match the %d total try jobs - %d skipped try jobs", ci.Issue, len(reuseSuccessTryJobs), len(tv.tasksCfg.CommitQueue), len(skippedTryJobs))
		}
		// If we are not waiting on anything and there were no failures then they were all successful.
		return SuccessState, fmt.Sprintf("%s Try jobs were successful: %s", tv.Name(), strings.Join(waitingTryJobs, ",")), nil
	}
}

// TODO(rmistry): This is left to be implemented completely.
func (tv *TryJobsVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo) {

	// providedPSID := tv.cr.GetEarliestEquivalentPatchSetID(ci)
	//Refresh the change to get the latest patchset ID.
	// tv.cr.GetIssueProperties(ci)
	// Then will have to FIND all the builds triggered by CQ (using a tag)
	// which are curently running and then cancel them by adding a new
	// method in buildbucket library.

	/*
		if tv.cr.GetEarliestEquivalentPatchSetID(ci) != patchSetID {
			// If the provided patchsetID is not the earliest equivalent patchset ID then
			// cleanup try jobs.
			fmt.Printf("\n\nTHE BUILDS OF %d need to BE CLEANED UP!!!!", ci.Issue)
		}
	*/

	return
}
