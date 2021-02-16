package gerrit

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func makeChangeInfo() *ChangeInfo {
	return &ChangeInfo{
		Labels: map[string]*LabelEntry{},
	}
}

func testConfig(t *testing.T, cfg *Config) {
	unittest.SmallTest(t)

	testEmpty(t, cfg)
	testCqInProgress(t, cfg)
	testCqSuccess(t, cfg)
	testCqSuccessNotMerged(t, cfg)
	testCqSuccessWithHigherValues(t, cfg)
	testCqFailed(t, cfg)
	testCqFailedWithLowerValues(t, cfg)
	testDryRunInProgress(t, cfg)
	testDryRunSuccess(t, cfg)
	testDryRunSuccessWithHigherValues(t, cfg)
	testDryRunFailed(t, cfg)
	testDryRunFailedWithLowerValues(t, cfg)
}

// Initial empty change. No CQ labels at all.
func testEmpty(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()

	require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	if cfg.HasCq {
		// Have to use false here because ANGLE and Chromium configs do not use
		// CQ success/failure labels, so we can't distinguish between "dry run
		// finished" and "dry run never started".
		require.False(t, cfg.DryRunSuccess(ci, false))
	} else {
		// DryRunSuccess is always true with no CQ.
		require.True(t, cfg.DryRunSuccess(ci, false))
	}
}

// CQ in progress.
func testCqInProgress(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetCqLabels)
	if cfg.HasCq {
		require.True(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.False(t, cfg.DryRunSuccess(ci, true))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// CQ success.
func testCqSuccess(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	if len(cfg.CqSuccessLabels) > 0 {
		SetLabels(ci, cfg.CqSuccessLabels)
	}
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.CqActiveLabels)
	}
	ci.Status = ChangeStatusMerged
	if cfg.HasCq {
		require.False(t, cfg.CqRunning(ci))
		require.True(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, false))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.True(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// CQ success with higher label values than specified.
func testCqSuccessWithHigherValues(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	labels := map[string]int{}
	if len(cfg.CqSuccessLabels) > 0 {
		for k, v := range cfg.CqSuccessLabels {
			labels[k] = v + 1
		}
	}
	SetLabels(ci, labels)
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.CqActiveLabels)
	}
	ci.Status = ChangeStatusMerged
	if cfg.HasCq {
		require.False(t, cfg.CqRunning(ci))
		require.True(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, false))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.True(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// CQ success but not merged yet (this is a race condition which occurs
// occasionally on the Android rollers).
func testCqSuccessNotMerged(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetCqLabels)
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.CqActiveLabels)
	}
	if len(cfg.CqSuccessLabels) > 0 {
		SetLabels(ci, cfg.CqSuccessLabels)
	}
	ci.Status = ""
	if cfg.HasCq {
		// In this case, we're waiting for the CQ to land the change, so
		// we consider it to still be running.
		if len(cfg.CqSuccessLabels) > 0 {
			require.True(t, cfg.CqRunning(ci))
			require.True(t, cfg.DryRunSuccess(ci, false))
		} else {
			require.False(t, cfg.CqRunning(ci))
			require.False(t, cfg.DryRunSuccess(ci, false))
		}
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// CQ failed.
func testCqFailed(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetCqLabels)
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.CqActiveLabels)
	}
	if len(cfg.CqFailureLabels) > 0 {
		SetLabels(ci, cfg.CqFailureLabels)
	}
	ci.Status = ""
	if cfg.HasCq {
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.False(t, cfg.DryRunSuccess(ci, false))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// CQ failed with lower label values than specified.
func testCqFailedWithLowerValues(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetCqLabels)
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.CqActiveLabels)
	}
	labels := map[string]int{}
	if len(cfg.CqFailureLabels) > 0 {
		for k, v := range cfg.CqFailureLabels {
			labels[k] = v - 1
		}
	}
	SetLabels(ci, labels)
	ci.Status = ""
	if cfg.HasCq {
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.False(t, cfg.DryRunSuccess(ci, false))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// Dry run in progress.
func testDryRunInProgress(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetDryRunLabels)
	if cfg.HasCq {
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.True(t, cfg.DryRunRunning(ci))
		require.False(t, cfg.DryRunSuccess(ci, true))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// Dry run success.
func testDryRunSuccess(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetDryRunLabels)
	if len(cfg.DryRunSuccessLabels) > 0 {
		SetLabels(ci, cfg.DryRunSuccessLabels)
	}
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.DryRunActiveLabels)
	}
	// Unfortunately, with no labels to differentiate, we can't verify that
	// CqRunning is false here.
	//require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	require.True(t, cfg.DryRunSuccess(ci, true))
}

// Dry run success with higher label values than specified.
func testDryRunSuccessWithHigherValues(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetDryRunLabels)
	labels := map[string]int{}
	if len(cfg.DryRunSuccessLabels) > 0 {
		for k, v := range cfg.DryRunSuccessLabels {
			labels[k] = v + 1
		}
	}
	SetLabels(ci, labels)
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.DryRunActiveLabels)
	}
	// Unfortunately, with no labels to differentiate, we can't verify that
	// CqRunning is false here.
	//require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	require.True(t, cfg.DryRunSuccess(ci, true))
}

// Dry run failed.
func testDryRunFailed(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetDryRunLabels)
	if len(cfg.DryRunFailureLabels) > 0 {
		SetLabels(ci, cfg.DryRunFailureLabels)
	}
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.DryRunActiveLabels)
	}
	if cfg.HasCq {
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.False(t, cfg.DryRunSuccess(ci, false))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// Dry run failed with lower label values than specified.
func testDryRunFailedWithLowerValues(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	SetLabels(ci, cfg.SetDryRunLabels)
	labels := map[string]int{}
	if len(cfg.DryRunFailureLabels) > 0 {
		for k, v := range cfg.DryRunFailureLabels {
			labels[k] = v - 1
		}
	}
	SetLabels(ci, labels)
	if cfg.CqLabelsUnsetOnCompletion {
		UnsetLabels(ci, cfg.DryRunActiveLabels)
	}
	if cfg.HasCq {
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.False(t, cfg.DryRunSuccess(ci, false))
	} else {
		// CQ and DryRun are never running with no CQ. CqSuccess is only
		// true if the change is merged, and DryRunSuccess is always
		// true.
		require.False(t, cfg.CqRunning(ci))
		require.False(t, cfg.CqSuccess(ci))
		require.False(t, cfg.DryRunRunning(ci))
		require.True(t, cfg.DryRunSuccess(ci, true))
	}
}

// Partial CQ success; in the case of Android, this occurs when the presubmit
// produces warnings, which causes the Verified+1 label to be set but the
// Autosubmit+1 label to be removed.
func testPartialCqSuccess(t *testing.T, cfg *Config) {
	ci := makeChangeInfo()
	labels := map[string]int{}
	for k, v := range cfg.CqSuccessLabels {
		labels[k] = v
	}
	for k, v := range cfg.NoCqLabels {
		labels[k] = v
	}
	SetLabels(ci, labels)
	require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
}

func TestConfigAndroid(t *testing.T) {
	cfg := ConfigAndroid
	testConfig(t, cfg)
	testPartialCqSuccess(t, cfg)
}

func TestConfigANGLE(t *testing.T) {
	testConfig(t, ConfigANGLE)
}

func TestConfigChromium(t *testing.T) {
	testConfig(t, ConfigChromium)
}

func TestConfigChromiumNoCQ(t *testing.T) {
	testConfig(t, ConfigChromiumNoCQ)
}

func TestConfigLibassistant(t *testing.T) {
	testConfig(t, ConfigLibAssistant)
}
