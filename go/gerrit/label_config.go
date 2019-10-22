package gerrit

var (
	CONFIG_ANDROID = &Config{
		SetCqLabels: map[string]interface{}{
			AUTOSUBMIT_LABEL:      AUTOSUBMIT_LABEL_SUBMIT,
			CODEREVIEW_LABEL:      CODEREVIEW_LABEL_SELF_APPROVE,
			PRESUBMIT_READY_LABEL: PRESUBMIT_READY_LABEL_ENABLE,
		},
		SetDryRunLabels: map[string]interface{}{
			AUTOSUBMIT_LABEL:      AUTOSUBMIT_LABEL_NONE,
			CODEREVIEW_LABEL:      CODEREVIEW_LABEL_SELF_APPROVE,
			PRESUBMIT_READY_LABEL: PRESUBMIT_READY_LABEL_ENABLE,
		},
		CqActiveLabels: map[string]interface{}{
			AUTOSUBMIT_LABEL:      AUTOSUBMIT_LABEL_SUBMIT,
			CODEREVIEW_LABEL:      CODEREVIEW_LABEL_SELF_APPROVE,
			PRESUBMIT_READY_LABEL: PRESUBMIT_READY_LABEL_ENABLE,
		},
		CqSuccessLabels: map[string]interface{}{
			PRESUBMIT_VERIFIED_LABEL: PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
		},
		CqFailureLabels: map[string]interface{}{
			PRESUBMIT_VERIFIED_LABEL: PRESUBMIT_VERIFIED_LABEL_REJECTED,
		},
		DryRunActiveLabels: map[string]interface{}{
			AUTOSUBMIT_LABEL:      AUTOSUBMIT_LABEL_NONE,
			CODEREVIEW_LABEL:      CODEREVIEW_LABEL_SELF_APPROVE,
			PRESUBMIT_READY_LABEL: PRESUBMIT_READY_LABEL_ENABLE,
		},
		DryRunSuccessLabels: map[string]interface{}{
			PRESUBMIT_VERIFIED_LABEL: PRESUBMIT_VERIFIED_LABEL_ACCEPTED,
		},
		DryRunFailureLabels: map[string]interface{}{
			PRESUBMIT_VERIFIED_LABEL: PRESUBMIT_VERIFIED_LABEL_REJECTED,
		},
		DryRunUsesTryjobResults: false,
	}

	CONFIG_ANGLE = &Config{
		SetCqLabels: map[string]interface{}{
			CODEREVIEW_LABEL:  CODEREVIEW_LABEL_SELF_APPROVE,
			COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_SUBMIT,
		},
		SetDryRunLabels: map[string]interface{}{
			CODEREVIEW_LABEL:  CODEREVIEW_LABEL_SELF_APPROVE,
			COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_DRY_RUN,
		},
		CqActiveLabels: map[string]interface{}{
			CODEREVIEW_LABEL:  CODEREVIEW_LABEL_SELF_APPROVE,
			COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_SUBMIT,
		},
		CqSuccessLabels: map[string]interface{}{},
		CqFailureLabels: map[string]interface{}{},
		DryRunActiveLabels: map[string]interface{}{
			CODEREVIEW_LABEL:  CODEREVIEW_LABEL_SELF_APPROVE,
			COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_DRY_RUN,
		},
		DryRunSuccessLabels:     map[string]interface{}{},
		DryRunFailureLabels:     map[string]interface{}{},
		DryRunUsesTryjobResults: true,
	}

	CONFIG_CHROMIUM = &Config{
		SetCqLabels: map[string]interface{}{
			CODEREVIEW_LABEL:  CODEREVIEW_LABEL_APPROVE,
			COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_SUBMIT,
		},
		SetDryRunLabels: map[string]interface{}{
			CODEREVIEW_LABEL:  CODEREVIEW_LABEL_APPROVE,
			COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_DRY_RUN,
		},
		CqActiveLabels: map[string]interface{}{
			CODEREVIEW_LABEL:  CODEREVIEW_LABEL_APPROVE,
			COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_SUBMIT,
		},
		CqSuccessLabels: map[string]interface{}{},
		CqFailureLabels: map[string]interface{}{},
		DryRunActiveLabels: map[string]interface{}{
			CODEREVIEW_LABEL:  CODEREVIEW_LABEL_APPROVE,
			COMMITQUEUE_LABEL: COMMITQUEUE_LABEL_DRY_RUN,
		},
		DryRunSuccessLabels:     map[string]interface{}{},
		DryRunFailureLabels:     map[string]interface{}{},
		DryRunUsesTryjobResults: true,
	}
)

type Config struct {
	// Labels to set to run the Commit Queue.
	SetCqLabels map[string]interface{}
	// Labels to set to run the Commit Queue in dry run mode.
	SetDryRunLabels map[string]interface{}

	// If the issue is open and all of these labels are set, the Commit
	// Queue is considered active.
	CqActiveLabels map[string]interface{}
	// If the issue is merged or all of these labels are set, the Commit
	// Queue is considered to have finished successfully.
	CqSuccessLabels map[string]interface{}
	// If the issue is abandoned or all of these labels are set, the Commit
	// Queue is considered to have failed.
	CqFailureLabels map[string]interface{}

	// If the issue is open and all of these labels are set, the dry run is
	// considered active.
	DryRunActiveLabels map[string]interface{}
	// If the issue is merged or all of these labels are set, the dry run is
	// considered to have finished successfuly.
	DryRunSuccessLabels map[string]interface{}
	// If the issue is abandoned or all of these labels are set, the dry run
	// is considered to have failed.
	DryRunFailureLabels map[string]interface{}
	// DryRunUsesTryjobResults is true if tryjob results should be factored
	// into dry run success.
	DryRunUsesTryjobResults bool
}

// all returns true iff all of the given label keys and values are set on the
// change. Returns true if the given map of labels is empty.
func all(ci *ChangeInfo, labels map[string]interface{}) bool {
	for labelKey, wantValue := range labels {
		found := false
		if labelEntry, ok := ci.Labels[labelKey]; ok {
			for _, labelDetail := range labelEntry.All {
				if wantValue == labelDetail.Value {
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

// CqRunning returns true if the commit queue is still running. Returns false if
// the change is merged or abandoned.
func (c *Config) CqRunning(ci *ChangeInfo) bool {
	if ci.IsClosed() {
		return false
	}
	if len(c.CqFailureLabels) > 0 && all(ci, c.CqFailureLabels) {
		return false
	}
	if len(c.CqSuccessLabels) > 0 && all(ci, c.CqSuccessLabels) {
		return false
	}
	if len(c.CqActiveLabels) > 0 && all(ci, c.CqActiveLabels) {
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
	if ci.IsClosed() {
		return false
	}
	if len(c.DryRunSuccessLabels) > 0 && all(ci, c.DryRunSuccessLabels) {
		return false
	}
	if len(c.DryRunFailureLabels) > 0 && all(ci, c.DryRunFailureLabels) {
		return false
	}
	if len(c.DryRunActiveLabels) > 0 && all(ci, c.DryRunActiveLabels) {
		return true
	}
	return false
}

// DryRunSuccess returns true if the dry run succeeded. The allTrybotsSucceeded
// parameter indicates whether or not all of the relevant trybots for this
// change succeeded; it is unused if Config.DryRunUsesTryjobResults is false.
func (c *Config) DryRunSuccess(ci *ChangeInfo, allTrybotsSucceeded bool) bool {
	if c.CqRunning(ci) || c.DryRunRunning(ci) {
		return false
	}
	if ci.IsClosed() {
		return ci.IsMerged()
	}
	if len(c.DryRunSuccessLabels) > 0 && all(ci, c.DryRunSuccessLabels) {
		return true
	}
	if len(c.DryRunFailureLabels) > 0 && all(ci, c.DryRunFailureLabels) {
		return false
	}
	return c.DryRunUsesTryjobResults && allTrybotsSucceeded
}
