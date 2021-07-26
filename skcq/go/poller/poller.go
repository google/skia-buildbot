package poller

// Initializes and polls the various issue frameworks.

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.skia.org/infra/go/gerrit"

	"go.skia.org/infra/go/util"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/caches"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/config"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/throttler"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/verifiers"
)

const (
	LivenessMetric = "skcq_be"
)

// Start polls Gerrit for matching dry-run/CQ issues, gets their verifiers,
// and runs them.
func Start(ctx context.Context, pollInterval time.Duration, cr codereview.CodeReview, currentChangesCache caches.CurrentChangesCache, httpClient, criaClient *http.Client, dbClient db.DB, canModifyCfgsOnTheFly *allowed.AllowedFromChromeInfraAuth, publicFEInstanceURL, corpFEInstanceURL string, reposAllowList, reposBlockList []string) error {
	liveness := metrics2.NewLiveness(LivenessMetric)
	tm := throttler.NewThrottler()
	vm := verifiers.NewSkCQVerifiersManager(tm, httpClient, criaClient, cr)
	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		sklog.Info("----------------New Poll Iteration--------------")
		cls, err := cr.Search(ctx)
		if err != nil {
			sklog.Errorf("Error when searching for issues: %s", err)
			return
		} else {
			liveness.Reset()
		}

		// Store CLs that are being processed in this round for quicker lookup.
		clsInThisRound := map[string]bool{}

		// Process the CLs.
		for _, incompleteCI := range cls {
			// Get the full issue properties of this change. This is done at the
			// start of the loop and not in cr.Search because right before we are
			// about to process a change we want the latest data available.
			ci, err := cr.GetIssueProperties(ctx, incompleteCI.Issue)
			if err != nil {
				sklog.Errorf("[%d] Could not get full issue properties: %s", incompleteCI.Issue, err)
				continue
			}

			// Skip changes with repos not in allow list or in block list.
			if len(reposAllowList) > 0 && !util.In(ci.Project, reposAllowList) {
				sklog.Infof("[%d] Ignoring change because the repo %s is not in the repos allowlist: %s.", ci.Issue, ci.Project, reposAllowList)
				continue
			}
			if len(reposBlockList) > 0 && util.In(ci.Project, reposBlockList) {
				sklog.Infof("[%d] Ignoring change because the repo %s is in the repos blocklist: %s.", ci.Issue, ci.Project, reposBlockList)
				continue
			}

			// Instantiate configReader.
			// TODO(rmistry): Cache these config readers per repo so that we do not
			// have to keep creating new ones.
			configReader, err := config.NewGitilesConfigReader(ctx, httpClient, ci, cr, canModifyCfgsOnTheFly)
			if err != nil {
				sklog.Errorf("[%d] Error when instantiating config reader: %s", ci.Issue, err)
				cr.RemoveFromCQ(ctx, ci, "Error when reading configs. Removing from CQ. Please ask Infra Gardener to investigate.")
				return
			}

			processCL(ctx, vm, ci, configReader, clsInThisRound, cr, currentChangesCache, httpClient, dbClient, canModifyCfgsOnTheFly, publicFEInstanceURL, corpFEInstanceURL, tm)
		}

		// Find CLs that were processed in the last cycle but not the current one.
		// These CLs might still be running, mark them as abandoned and run cleanup
		// on their verifiers.
		for changeEquivalentPatchset, cqRecord := range currentChangesCache.Get() {
			if _, ok := clsInThisRound[changeEquivalentPatchset]; !ok {

				ci, err := cr.GetIssueProperties(ctx, cqRecord.ChangeID)
				if err != nil {
					sklog.Errorf("[%d] Could not get issue properties during cleanup: %s", cqRecord.ChangeID, err)
					continue
				}

				configReader, err := config.NewGitilesConfigReader(ctx, httpClient, ci, cr, canModifyCfgsOnTheFly)
				if err != nil {
					sklog.Errorf("[%d] Could not get config reader during cleanup: %s", ci.Issue, err)
					continue
				}

				if err := cleanupCL(ctx, changeEquivalentPatchset, currentChangesCache, dbClient, cqRecord, ci, configReader, cr, httpClient, vm); err != nil {
					sklog.Errorf("[%d] Error when cleaning up %s: %s", ci.Issue, changeEquivalentPatchset, err)
					continue
				}
			}
		}

	}, nil)

	return nil
}

func processCL(ctx context.Context, vm types.VerifiersManager, ci *gerrit.ChangeInfo, configReader config.ConfigReader, clsInThisRound map[string]bool, cr codereview.CodeReview, currentChangesCache caches.CurrentChangesCache, httpClient *http.Client, dbClient db.DB, canModifyCfgsOnTheFly allowed.Allow, publicFEInstanceURL, corpFEInstanceURL string, tm types.ThrottlerManager) {

	// Make sure the change is still open and has either CQ+1 and CQ+2.
	if ci.IsClosed() || !(cr.IsDryRun(ctx, ci) || cr.IsCQ(ctx, ci)) {
		sklog.Infof("[%d] Ignoring change because it is no longer open or does not have the CQ+1/CQ+2 votes.", ci.Issue)
		return
	}

	// Use the equivalent patchset for the changes cache because
	// if NO_CODE change patches come in then we want to treat it the
	// same as the earliest equivalent patch.
	changeEquivalentPatchset := fmt.Sprintf("%d/%d", ci.Issue, cr.GetEarliestEquivalentPatchSetID(ci))
	clsInThisRound[changeEquivalentPatchset] = true

	repoBranch := fmt.Sprintf("%s/%s", ci.Project, ci.Branch)
	sklog.Infof("[%d] Started processing in repo+branch %s", ci.Issue, repoBranch)

	// Get the SkCQ cfg that will be used for this change.
	skCQCfg, err := configReader.GetSkCQCfg(ctx)
	if err != nil {
		sklog.Infof("[%d] Error when reading %s: %s", ci.Issue, config.SkCQCfgPath, err)
		if config.IsNotFound(err) {
			cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("%s. Removing from CQ.\nPlease add a %s file if this repo+branch requires CQ.", err.Error(), config.SkCQCfgPath))
			return
		} else if config.IsCannotModifyCfgsOnTheFly(err) {
			cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("CL owner %s does not have permission to modify %s", ci.Owner.Email, config.SkCQCfgPath))
			return
		} else {
			cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("Error reading %s for %d. Removing from CQ. Please ask Infra Gardener to investigate.", config.SkCQCfgPath, ci.Issue))
			return
		}
	}

	// Is this a change from an internal repo?
	internalRepo := skCQCfg.VisibilityType == config.InternalVisibility

	// Gather all verifiers that will be used and all changes that will be
	// submitted at the same time as this change.
	clVerifiers, togetherChanges, err := vm.GetVerifiers(ctx, skCQCfg, ci, false /* isSubmittedTogetherChange */, configReader)
	if err != nil {
		sklog.Errorf("[%d] Error when getting verifiers: %s", ci.Issue, err)
		cr.RemoveFromCQ(ctx, ci, "Error when getting verifiers. Removing from CQ. Please ask Infra Gardener to investigate.")
		return
	}
	// Log verifiers.
	verifierNames := []string{}
	for _, clVerifier := range clVerifiers {
		verifierNames = append(verifierNames, clVerifier.Name())
	}
	sklog.Infof("[%d] uses verifiers: %s", ci.Issue, strings.Join(verifierNames, ", "))

	// Update the cache if it is not already in there.
	cqStartTime, newCQRun, err := currentChangesCache.Add(ctx, changeEquivalentPatchset, ci.Subject, ci.Owner.Email, ci.Project, ci.Branch, !cr.IsCQ(ctx, ci), internalRepo, ci.Issue, cr.GetLatestPatchSetID(ci))
	if err != nil {
		sklog.Errorf("[%d] could not update the currentChangesCache: %s", ci.Issue, err)
	}
	cqEndTime := int64(0)
	cqSubmittedTime := int64(0)

	if newCQRun {
		// If this is a new CQ run then before running the verifiers update the
		// CL with an auto-generated comment saying we are processing this patch.
		feURL := publicFEInstanceURL
		if internalRepo {
			feURL = corpFEInstanceURL
		}
		notify := gerrit.NotifyNone
		comment := "SkCQ is trying the patch."
		if !cr.IsCQ(ctx, ci) {
			comment = fmt.Sprintf("Dry run: %s", comment)
		} else if len(togetherChanges) > 0 {
			togetherChangesLinks := []string{}
			for _, t := range togetherChanges {
				togetherChangesLinks = append(togetherChangesLinks, fmt.Sprintf("https://skia-review.googlesource.com/c/%s", t))
			}
			comment = fmt.Sprintf("%s\nThis change will be submitted with the following changes: %s", comment, strings.Join(togetherChangesLinks, ", "))
			// Notify owner and reviewers to let them know that other changes might be submitted as well.
			notify = gerrit.NotifyOwnerReviewers
		}
		comment = fmt.Sprintf("%s\n\nFollow status at: %s/%d/%d", comment, feURL, ci.Issue, cr.GetLatestPatchSetID(ci))
		if err := cr.AddComment(ctx, ci, comment, notify); err != nil {
			sklog.Errorf("[%d] Could not add started processing comment: %s", ci.Issue, err)
		}
	}

	// Now run the verifiers.
	verifierStatuses := vm.RunVerifiers(ctx, ci, clVerifiers, cqStartTime)
	rejectMsgsFromVerifiers, waitMsgsFromVerifiers, successMsgsFromVerifiers := verifiers.GetStatusStringsFromVerifierStatuses(verifierStatuses)
	var attemptOverallState types.VerifierState
	if len(rejectMsgsFromVerifiers) > 0 {
		// There were failed verifiers.
		sklog.Infof("[%d] from %s has failed verifiers: %+v", ci.Issue, repoBranch, rejectMsgsFromVerifiers)
		cr.RemoveFromCQ(ctx, ci, fmt.Sprintf("Removing from SkCQ because verifiers have failed:\n\n%s", strings.Join(rejectMsgsFromVerifiers, "\n")))
		if err := currentChangesCache.Remove(ctx, changeEquivalentPatchset); err != nil {
			sklog.Errorf("[%d] could not update the currentChangesCache: %s", ci.Issue, err)
		}
		attemptOverallState = types.VerifierFailureState
		cqEndTime = time.Now().Unix()
	} else if len(waitMsgsFromVerifiers) > 0 {
		// There are verifiers we need to wait for.
		sklog.Infof("[%d] from %s is waiting for verifiers: %s", ci.Issue, repoBranch, strings.Join(waitMsgsFromVerifiers, ", "))
		attemptOverallState = types.VerifierWaitingState
	} else {
		// There were no failed verifiers or verifiers that we need to wait for
		sklog.Infof("[%d] from %s successfully ran verifiers: %s", ci.Issue, repoBranch, strings.Join(successMsgsFromVerifiers, ", "))
		if !cr.IsCQ(ctx, ci) {
			removeFromCQMsg := "Dry run: This CL passed the SkCQ dry run."
			if ci.WorkInProgress {
				// If the change is WIP and a reviewer has been added, then
				// automatically remove the change from WIP by publishing it.
				for _, r := range ci.Reviewers.Reviewer {
					// Owner shows up as a reviewer. No idea why Gerrit does this.
					if r.AccountID != ci.Owner.AccountID {
						// Publish and break out.
						if err := cr.SetReadyForReview(ctx, ci); err != nil {
							sklog.Errorf("[%d] Could not set ready for review: %s", ci.Issue, err)
						}
						removeFromCQMsg += "\nAutomatically published the CL because it was WIP with reviewers specified."
						break
					}
				}
			}
			// Say everything was succesful and we are done.
			cr.RemoveFromCQ(ctx, ci, removeFromCQMsg)
		} else {
			if err := cr.Submit(ctx, ci); err != nil {
				sklog.Errorf("[%d] Error when submitting: %s", ci.Issue, err)
				cr.RemoveFromCQ(ctx, ci, "Error when submitting. Removing from SkCQ. Please ask Infra Gardener to investigate.")
			} else {
				cqSubmittedTime = time.Now().Unix()
				tm.UpdateThrottler(repoBranch, time.Now(), skCQCfg.ThrottlerCfg)
			}
		}
		if err := currentChangesCache.Remove(ctx, changeEquivalentPatchset); err != nil {
			sklog.Errorf("[%d] could not update the currentChangesCache: %s", ci.Issue, err)
		}
		cqEndTime = time.Now().Unix()
		attemptOverallState = types.VerifierSuccessState
	}

	if attemptOverallState == types.VerifierFailureState {
		// Do a pass through of all verifiers and update any states that are in
		// VerifierWatitingState to VerifierAbortedState because we are no longer
		// waiting for this verifier.
		for _, v := range verifierStatuses {
			if v.State == types.VerifierWaitingState {
				v.State = types.VerifierAbortedState
				v.StopTs = time.Now().Unix()
			}
		}
	}

	// We are done processing this CL for this iteration of the poller's
	// loop. Persist the CL state for UI display.
	attempt := &types.ChangeAttempt{
		ChangeID:           ci.Issue,
		PatchsetID:         cr.GetLatestPatchSetID(ci),
		DryRun:             !cr.IsCQ(ctx, ci),
		Repo:               ci.Project,
		Branch:             ci.Branch,
		PatchStartTs:       cqStartTime,
		PatchStopTs:        cqEndTime,
		PatchCommittedTs:   cqSubmittedTime,
		SubmittableChanges: togetherChanges,
		VerifiersStatuses:  verifierStatuses,
		OverallState:       attemptOverallState,
	}
	if err := dbClient.PutChangeAttempt(ctx, attempt, db.GetChangesCol(internalRepo)); err != nil {
		sklog.Errorf("[%d] Could not persist change attempt: %s", ci.Issue, err)
		return
	}
}

func cleanupCL(ctx context.Context, changeEquivalentPatchset string, currentChangesCache caches.CurrentChangesCache, dbClient db.DB, cqRecord *types.CurrentlyProcessingChange, ci *gerrit.ChangeInfo, configReader config.ConfigReader, cr codereview.CodeReview, httpClient *http.Client, vm types.VerifiersManager) error {
	sklog.Infof("[%d] %s is no longer processed by SkCQ. It was processed in the last cycle and was still running. Going to mark it as abandoned and cleanup it's verifiers.", cqRecord.ChangeID, changeEquivalentPatchset)

	// Remove the change from the changes cache.
	if err := currentChangesCache.Remove(ctx, changeEquivalentPatchset); err != nil {
		return skerr.Wrapf(err, "[%d] could not update the currentChangesCache during cleanup", cqRecord.ChangeID)
	}

	// Update the attempt as being abandoned so that the UI accurately reflects what happened to
	// the change.
	if err := dbClient.UpdateChangeAttemptAsAbandoned(ctx, cqRecord.ChangeID, cqRecord.LatestPatchsetID, db.GetChangesCol(cqRecord.Internal), cqRecord.StartTs); err != nil {
		return skerr.Wrapf(err, "[%d] Could not mark change %s as abandoned during cleanup", cqRecord.ChangeID, changeEquivalentPatchset)
	}

	// Instantiate all objs needed to get verifiers.
	skCQCfg, err := configReader.GetSkCQCfg(ctx)
	if err != nil {
		return skerr.Wrapf(err, "[%d] Could not get %s during cleanup: %s", ci.Issue, config.SkCQCfgPath, err)
	}
	// Get all verifiers.
	verifiers, _, err := vm.GetVerifiers(ctx, skCQCfg, ci, false, configReader)
	if err != nil {
		return skerr.Wrapf(err, "[%d] Could not get verifiers to cleanup", ci.Issue)
	}
	// Parse out the previous patchset from the changeEquivalentPatchset.
	tokens := strings.Split(changeEquivalentPatchset, "/")
	previousPatchsetID, err := strconv.ParseInt(tokens[1], 10, 64)
	if err != nil {
		return skerr.Wrapf(err, "[%d] Could not parse patchsetID from %s", ci.Issue, tokens[1])
	}
	// Run cleanup on all verifiers.
	for _, v := range verifiers {
		v.Cleanup(ctx, ci, previousPatchsetID)
	}
	return nil
}
