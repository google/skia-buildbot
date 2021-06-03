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
	TryJobStaleTimeoutSecs = 24 * 60 * 60
	// TryJobStaleTimeoutSecs = 2 * 60
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
func (tv *TryJobsVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo) (state VerifierState, reason string, err error) {

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
	gerritURL := u.Host
	latestPatchSetID := tv.cr.GetLatestPatchSetID(ci)
	tryJobsOnChange, err := tv.bb2.GetTrybotsForCL(ctx, ci.Issue, latestPatchSetID, issueURL)
	if err != nil {
		return "", "", skerr.Fmt("Could not get tryjobs for %d: %s", ci.Issue, err)
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
	// C

	// Cross reference try jobs on the change with CQ tryjobs.
	staleTryJobs := []string{}
	reuseTryJobs := []string{}
	notFoundTryJobs := []string{}
	for cqJobName, cqCfg := range tv.tasksCfg.CommitQueue {

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
						break regexesLoop
					}
				}
			}
			if !locationRegexMatch {
				// Ignore this CQ job.
				sklog.Infof("[%d] Skipping CQ job %s because it did not match any of the location regexes", ci.Issue, cqJobName)
				continue
			}
		}

		// Is the job already triggered on the change?
		if build, ok := nameToBuilderOnChange[cqJobName]; ok {
			fmt.Println("LOOKING AT STATUS NOW!!!!")
			fmt.Println(cqJobName)
			fmt.Println(build.GetStatus())
			// Make sure the build is not Stale.
			if time.Now().Unix()-build.GetCreateTime().GetSeconds() >= TryJobStaleTimeoutSecs {
				staleTryJobs = append(staleTryJobs, cqJobName)
			} else {
				reuseTryJobs = append(reuseTryJobs, cqJobName)
			}
		} else {
			notFoundTryJobs = append(notFoundTryJobs, cqJobName)
		}
	}

	// Go through reuseTryJobs and see if they are successful or failed.
	// buildbucketpb.Status_SUCCESS
	// Problem here is I do not know how to tell what should be considered a failure or should be something else....
	// Will need to schedule some sort of a tag...

	// Do logging here!!!
	sklog.Infof("[%d] For CQ try jobs- Found %d stale try jobs, %d reusable try jobs, %d try jobs were not found, out of the required %d cq try jobs", ci.Issue, len(staleTryJobs), len(reuseTryJobs), len(notFoundTryJobs), len(tv.tasksCfg.CommitQueue))
	triggerTryJobs := append(staleTryJobs, notFoundTryJobs...)

	if len(triggerTryJobs) > 0 {
		fmt.Println("SCHEDULING BUILDS!")
		// Make configurable!
		respBuilds, err := tv.bb2.ScheduleBuilds(ctx, triggerTryJobs, ci.Issue, latestPatchSetID, gerritURL, ci.Project, "skia", "skia.testing")
		if err != nil {
			fmt.Println("ERROR SCHEDULING GUILDS!!!")
			return "", "", skerr.Fmt("Could not trigger %+v tryjobs for %d: %s", triggerTryJobs, ci.Issue, err)
		}
		// Make sure the builds were triggered? hopefully this step is unncessary.
		newTryJobsOnChange, err := tv.bb2.GetTrybotsForCL(ctx, ci.Issue, latestPatchSetID, issueURL)
		if err != nil {
			return "", "", skerr.Fmt("Could not get tryjobs for %d: %s", ci.Issue, err)
		}
		fmt.Println("NEW")
		fmt.Println(len(respBuilds))
		fmt.Println(len(newTryJobsOnChange))
		for _, b := range respBuilds {
			found := false
			// Make sure this build is in the new tryjobs on change.
			for _, n := range newTryJobsOnChange {
				if b.GetId() == n.GetId() {
					found = true
				}
			}
			if !found {
				return "", "", skerr.Fmt("%s with id %d was scheduled but did not show up on buildbucket", b.GetBuilder().GetBuilder(), b.GetId())
			}
		}

	} else {
		// All CQ tryjobs have already been triggered.
		// Check for their status now! IGNORE EXPERIMENTAL STUFF!

		// Return success or failure here based on that.
	}

	// Return waiting here!

	// If stale or trigger where >0 then do not bother checking for success
	// only check for success in the else case
	// If some where triggered then get tryjobsOnChange again? or wait?

	// When checking for tryjobs look for experimental stuff

	// Have Stale and Reuse.
	/**
		for bs in build_summaries:
	      # TODO(tandrii): categorize these builders earlier, when creating
	      # BuildSummary objects.
	      if current_time - bs.created_ts >= _TRYJOB_STALE_TIMEOUT:
	        # Build must be created recently enough for CQ to use it.
	        not_fresh.append(int(bs.build_id))
			**/

	// patchsetIDs := ci.GetPatchsetIDs()
	// latestPatchsetID := patchsetIDs[len(patchsetIDs)-1]
	// av.bb2.GetTrybotsForCL(ctx, ci.Issue, ci.GetLa)

	// Trigger them (using experimental stuff) and the keywords in the Footers!

	// Do not check for success/failure of experimental builds..

	return FailureState, fmt.Sprintf("%s This CL somethingsomethingsomething", tv.Name()), nil
}

func (tv *TryJobsVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo) {
	return
}
