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
	ci := makeChangeInfo()

	// Initial empty change. No CQ labels at all.
	require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	// Have to use false here because ANGLE and Chromium configs do not use
	// CQ success/failure labels, so we can't distinguish between "dry run
	// finished" and "dry run never started".
	require.False(t, cfg.DryRunSuccess(ci, false))

	// CQ in progress.
	SetLabels(ci, cfg.SetCqLabels)
	require.True(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	require.False(t, cfg.DryRunSuccess(ci, true))

	// CQ success.
	if len(cfg.CqSuccessLabels) > 0 {
		SetLabels(ci, cfg.CqSuccessLabels)
	}
	UnsetLabels(ci, cfg.CqActiveLabels)
	ci.Status = CHANGE_STATUS_MERGED
	require.False(t, cfg.CqRunning(ci))
	require.True(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	require.True(t, cfg.DryRunSuccess(ci, false))

	// CQ failed.
	if len(cfg.CqSuccessLabels) > 0 {
		UnsetLabels(ci, cfg.CqSuccessLabels)
	}
	if len(cfg.CqFailureLabels) > 0 {
		SetLabels(ci, cfg.CqFailureLabels)
	}
	ci.Status = ""
	require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	require.False(t, cfg.DryRunSuccess(ci, false))

	// Dry run in progress.
	if len(cfg.CqFailureLabels) > 0 {
		UnsetLabels(ci, cfg.CqFailureLabels)
	}
	UnsetLabels(ci, cfg.SetCqLabels)
	SetLabels(ci, cfg.SetDryRunLabels)
	require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.True(t, cfg.DryRunRunning(ci))
	require.False(t, cfg.DryRunSuccess(ci, true))

	// Dry run success.
	if len(cfg.DryRunSuccessLabels) > 0 {
		SetLabels(ci, cfg.DryRunSuccessLabels)
	}
	UnsetLabels(ci, cfg.DryRunActiveLabels)
	require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	require.True(t, cfg.DryRunSuccess(ci, true))

	// Dry run failed.
	if len(cfg.DryRunSuccessLabels) > 0 {
		UnsetLabels(ci, cfg.DryRunSuccessLabels)
	}
	if len(cfg.DryRunFailureLabels) > 0 {
		SetLabels(ci, cfg.DryRunFailureLabels)
	}
	require.False(t, cfg.CqRunning(ci))
	require.False(t, cfg.CqSuccess(ci))
	require.False(t, cfg.DryRunRunning(ci))
	require.False(t, cfg.DryRunSuccess(ci, false))
}

func TestConfigAndroid(t *testing.T) {
	testConfig(t, CONFIG_ANDROID)
}

func TestConfigANGLE(t *testing.T) {
	testConfig(t, CONFIG_ANGLE)
}

func TestConfigChromium(t *testing.T) {
	testConfig(t, CONFIG_CHROMIUM)
}
