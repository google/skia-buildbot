package gerrit

var (
	// ConfigAndroid is the configuration for Android Gerrit hosts.
	ConfigAndroid = &Config{
		SelfApproveLabels: map[string]int{
			LabelCodeReview: LabelCodeReviewSelfApprove,
		},
		HasCq: true,
		SetCqLabels: map[string]int{
			LabelAutoSubmit:     LabelAutoSubmitSubmit,
			LabelPresubmitReady: LabelPresubmitReadyEnable,
		},
		SetDryRunLabels: map[string]int{
			LabelAutoSubmit:     LabelAutoSubmitNone,
			LabelPresubmitReady: LabelPresubmitReadyEnable,
		},
		NoCqLabels: map[string]int{
			LabelAutoSubmit:     LabelAutoSubmitNone,
			LabelPresubmitReady: LabelPresubmitReadyNone,
		},
		CqActiveLabels: map[string]int{
			LabelAutoSubmit:     LabelAutoSubmitSubmit,
			LabelPresubmitReady: LabelPresubmitReadyEnable,
		},
		CqSuccessLabels: map[string]int{
			LabelAutoSubmit:        LabelAutoSubmitSubmit,
			LabelPresubmitVerified: LabelPresubmitVerifiedAccepted,
		},
		CqFailureLabels: map[string]int{
			LabelAutoSubmit:        LabelAutoSubmitSubmit,
			LabelPresubmitVerified: LabelPresubmitVerifiedRejected,
		},
		CqLabelsUnsetOnCompletion: true,
		DryRunActiveLabels: map[string]int{
			LabelAutoSubmit:     LabelAutoSubmitNone,
			LabelPresubmitReady: LabelPresubmitReadyEnable,
		},
		DryRunSuccessLabels: map[string]int{
			LabelPresubmitVerified: LabelPresubmitVerifiedAccepted,
		},
		DryRunFailureLabels: map[string]int{
			LabelPresubmitVerified: LabelPresubmitVerifiedRejected,
		},
		DryRunUsesTryjobResults: false,
	}

	// ConfigANGLE is the configuration for ANGLE Gerrit hosts.
	ConfigANGLE = &Config{
		SelfApproveLabels: map[string]int{
			LabelCodeReview: LabelCodeReviewSelfApprove,
		},
		HasCq: true,
		SetCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueSubmit,
		},
		SetDryRunLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueDryRun,
		},
		NoCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueNone,
		},
		CqActiveLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueSubmit,
		},
		CqSuccessLabels:           map[string]int{},
		CqFailureLabels:           map[string]int{},
		CqLabelsUnsetOnCompletion: true,
		DryRunActiveLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueDryRun,
		},
		DryRunSuccessLabels:     map[string]int{},
		DryRunFailureLabels:     map[string]int{},
		DryRunUsesTryjobResults: true,
	}

	// ConfigChromium is the configuration for Chromium Gerrit hosts.
	ConfigChromium = &Config{
		SelfApproveLabels: map[string]int{
			LabelCodeReview: LabelCodeReviewApprove,
		},
		HasCq: true,
		SetCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueSubmit,
		},
		SetDryRunLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueDryRun,
		},
		NoCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueNone,
		},
		CqActiveLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueSubmit,
		},
		CqSuccessLabels:           map[string]int{},
		CqFailureLabels:           map[string]int{},
		CqLabelsUnsetOnCompletion: true,
		DryRunActiveLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueDryRun,
		},
		DryRunSuccessLabels:     map[string]int{},
		DryRunFailureLabels:     map[string]int{},
		DryRunUsesTryjobResults: true,
	}

	// ConfigChromiumBotCommit is the configuration for Chromium Gerrit hosts
	// which use the Bot-Commit label instead of Code-Review.
	ConfigChromiumBotCommit = &Config{
		SelfApproveLabels: map[string]int{
			LabelBotCommit: LabelBotCommitApproved,
		},
		HasCq: true,
		SetCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueSubmit,
			LabelBotCommit:   LabelBotCommitApproved,
		},
		SetDryRunLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueDryRun,
			LabelBotCommit:   LabelBotCommitApproved,
		},
		NoCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueNone,
		},
		CqActiveLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueSubmit,
		},
		CqSuccessLabels:           map[string]int{},
		CqFailureLabels:           map[string]int{},
		CqLabelsUnsetOnCompletion: true,
		DryRunActiveLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueDryRun,
		},
		DryRunSuccessLabels:     map[string]int{},
		DryRunFailureLabels:     map[string]int{},
		DryRunUsesTryjobResults: true,
	}

	// ConfigChromiumNoCQ is the configuration for Chromium Gerrit hosts which
	// have no commit queue.
	ConfigChromiumNoCQ = &Config{
		SelfApproveLabels: map[string]int{
			LabelCodeReview: LabelCodeReviewApprove,
		},
		HasCq:           false,
		SetCqLabels:     map[string]int{},
		SetDryRunLabels: map[string]int{},
		NoCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueNone,
		},
		CqActiveLabels:            map[string]int{},
		CqSuccessLabels:           map[string]int{},
		CqFailureLabels:           map[string]int{},
		CqLabelsUnsetOnCompletion: true,
		DryRunActiveLabels:        map[string]int{},
		DryRunSuccessLabels:       map[string]int{},
		DryRunFailureLabels:       map[string]int{},
		DryRunUsesTryjobResults:   false,
	}

	// ConfigChromiumBotCommitNoCQ is the configuration for Chromium Gerrit
	// hosts which have no commit queue.
	ConfigChromiumBotCommitNoCQ = &Config{
		SelfApproveLabels: map[string]int{
			LabelCodeReview: LabelBotCommitApproved,
		},
		HasCq:           false,
		SetCqLabels:     map[string]int{},
		SetDryRunLabels: map[string]int{},
		NoCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueNone,
		},
		CqActiveLabels:            map[string]int{},
		CqSuccessLabels:           map[string]int{},
		CqFailureLabels:           map[string]int{},
		CqLabelsUnsetOnCompletion: true,
		DryRunActiveLabels:        map[string]int{},
		DryRunSuccessLabels:       map[string]int{},
		DryRunFailureLabels:       map[string]int{},
		DryRunUsesTryjobResults:   false,
	}

	// ConfigLibAssistant is the configuration for LibAssistant Gerrit hosts.
	ConfigLibAssistant = &Config{
		SelfApproveLabels: map[string]int{
			LabelCodeReview: LabelCodeReviewSelfApprove,
		},
		HasCq: true,
		SetCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueSubmit,
		},
		SetDryRunLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueDryRun,
		},
		NoCqLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueNone,
		},
		CqActiveLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueSubmit,
		},
		CqSuccessLabels: map[string]int{
			LabelVerified: LabelVerifiedAccepted,
		},
		CqFailureLabels: map[string]int{
			LabelVerified: LabelVerifiedRejected,
		},
		CqLabelsUnsetOnCompletion: false,
		DryRunActiveLabels: map[string]int{
			LabelCommitQueue: LabelCommitQueueDryRun,
		},
		DryRunSuccessLabels: map[string]int{
			LabelVerified: LabelVerifiedAccepted,
		},
		DryRunFailureLabels: map[string]int{
			LabelVerified: LabelVerifiedRejected,
		},
		DryRunUsesTryjobResults: false,
	}
)

// Config describes the configuration for a Gerrit host.
// TODO(borenet): Consider making Config into an interface with function calls
// for the various sets of labels and logic below. This format nicely groups the
// logic into one place but makes it difficult to understand and change.
type Config struct {
	// Labels to set to self-approve a change. For some projects this is the
	// same as a normal approval.
	SelfApproveLabels map[string]int

	// Whether or not this project has a Commit Queue.
	HasCq bool

	// Labels to set to run the Commit Queue.
	SetCqLabels map[string]int
	// Labels to set to run the Commit Queue in dry run mode.
	SetDryRunLabels map[string]int
	// Labels to set to remove from the Commit Queue.
	NoCqLabels map[string]int

	// If the issue is open and all of these labels are set, the Commit
	// Queue is considered active.
	CqActiveLabels map[string]int
	// If the issue is merged or all of these labels are set, the Commit
	// Queue is considered to have finished successfully.
	CqSuccessLabels map[string]int
	// If the issue is abandoned or all of these labels are set, the Commit
	// Queue is considered to have failed.
	CqFailureLabels map[string]int
	// CqLabelsUnsetOnCompletion is true if the commit queue unsets the
	// labels when it finishes.
	CqLabelsUnsetOnCompletion bool

	// If the issue is open and all of these labels are set, the dry run is
	// considered active.
	DryRunActiveLabels map[string]int
	// If the issue is merged or all of these labels are set, the dry run is
	// considered to have finished successfuly.
	DryRunSuccessLabels map[string]int
	// If the issue is abandoned or all of these labels are set, the dry run
	// is considered to have failed.
	DryRunFailureLabels map[string]int
	// DryRunUsesTryjobResults is true if tryjob results should be factored
	// into dry run success.
	DryRunUsesTryjobResults bool
}

// all returns true iff all of the given label keys and values are set on the
// change. Label value comparison is done using the specified compFunc.
// Returns true if the given map of labels is empty.
func all(ci *ChangeInfo, labels map[string]int, compFunc func(int, int) bool) bool {
	for labelKey, wantValue := range labels {
		found := false
		if labelEntry, ok := ci.Labels[labelKey]; ok {
			for _, labelDetail := range labelEntry.All {
				if compFunc(labelDetail.Value, wantValue) {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Comparison functions that can be used to pass to above all function.
func leq(x, y int) bool {
	return x <= y
}
func geq(x, y int) bool {
	return x >= y
}
func eq(x, y int) bool {
	return x == y
}

// MergeLabels returns a new map containing both sets of labels. Labels from the
// second set overwrite matching labels from the first.
func MergeLabels(a, b map[string]int) map[string]int {
	rv := make(map[string]int, len(a)+len(b))
	for k, v := range a {
		rv[k] = v
	}
	for k, v := range b {
		rv[k] = v
	}
	return rv
}

// CqRunning returns true if the commit queue is still running. Returns false if
// the change is merged or abandoned.
func (c *Config) CqRunning(ci *ChangeInfo) bool {
	if !c.HasCq {
		return false
	}
	if ci.IsClosed() {
		return false
	}
	// CqSuccess is only true if the change is merged, so if CqSuccessLabels
	// are set but the change is not yet merged, we have to consider the CQ
	// to be running or we'll incorrectly mark the CQ as failed. Note that
	// if the CQ never manages to merge the change, we'll be stuck in this
	// "CQ running even though it's finished" state indefinitely.
	if len(c.CqSuccessLabels) > 0 && all(ci, c.CqSuccessLabels, geq) {
		return true
	}
	if len(c.CqFailureLabels) > 0 && all(ci, c.CqFailureLabels, leq) {
		return false
	}
	if len(c.CqActiveLabels) > 0 && all(ci, c.CqActiveLabels, eq) {
		return true
	}
	return false
}

// CqSuccess returns true if the commit queue has finished successfully. This
// requires that the change is merged; CqSuccess returns false if the change is
// not merged, even if the commit queue finishes (ie. the relevant label is
// removed) and all trybots were successful or a CqSuccessLabel is applied.
func (c *Config) CqSuccess(ci *ChangeInfo) bool {
	return ci.IsMerged()
}

// DryRunRunning returns true if the dry run is still running. Returns false if
// the change is merged or abandoned.
func (c *Config) DryRunRunning(ci *ChangeInfo) bool {
	if !c.HasCq {
		return false
	}
	if ci.IsClosed() {
		return false
	}
	if len(c.DryRunActiveLabels) > 0 && !all(ci, c.DryRunActiveLabels, eq) {
		return false
	}
	if c.CqLabelsUnsetOnCompletion {
		return true
	}
	if len(c.DryRunSuccessLabels) > 0 && all(ci, c.DryRunSuccessLabels, geq) {
		return false
	} else if len(c.DryRunFailureLabels) > 0 && all(ci, c.DryRunFailureLabels, leq) {
		return false
	}
	return true
}

// DryRunSuccess returns true if the dry run succeeded. The allTrybotsSucceeded
// parameter indicates whether or not all of the relevant trybots for this
// change succeeded; it is unused if Config.DryRunUsesTryjobResults is false.
func (c *Config) DryRunSuccess(ci *ChangeInfo, allTrybotsSucceeded bool) bool {
	if ci.IsClosed() {
		return ci.IsMerged()
	}
	if !c.HasCq {
		// DryRunSuccess indicates that the CL has passed all of the
		// checks required for submission; if there are no checks, then
		// it has passed all of them by default.
		return true
	}
	if c.CqLabelsUnsetOnCompletion {
		if len(c.CqActiveLabels) > 0 && all(ci, c.CqActiveLabels, eq) {
			return false
		}
		if len(c.DryRunActiveLabels) > 0 && all(ci, c.DryRunActiveLabels, eq) {
			return false
		}
	}
	if len(c.DryRunSuccessLabels) > 0 && all(ci, c.DryRunSuccessLabels, geq) {
		return true
	}
	if len(c.DryRunFailureLabels) > 0 && all(ci, c.DryRunFailureLabels, leq) {
		return false
	}
	return c.DryRunUsesTryjobResults && allTrybotsSucceeded
}
