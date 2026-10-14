package internal

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"
)

func TestCollectBrowserVersionsActivity_NilCAS_ReturnsDescriptiveError(t *testing.T) {
	cr := &CommitRun{
		Runs: []*workflows.TestRun{
			{
				TaskID: "task-missing-stp",
				Status: run_benchmark.State(backends.RunBenchmarkFailure),
				CAS:    nil,
			},
		},
	}

	res, err := CollectBrowserVersionsActivity(context.Background(), "safari", "mac", cr)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "browser_versions task task-missing-stp finished with status BENCHMARK_FAILURE and no CAS output")
}

func TestSendCbbStpDownloadErrorEmail_DispatchesDiagnosticAlertEmail(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var capturedTo []string
	var capturedSubject, capturedBody string
	env.OnActivity(SendEmailActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedTo = args.Get(1).([]string)
		capturedSubject = args.Get(2).(string)
		capturedBody = args.Get(3).(string)
	}).Return(nil).Once()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		sendCbbStpDownloadErrorEmail(ctx, errors.New("missing <dmg> link & error"))
		return nil
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, []string{errorNotificationEmail}, capturedTo)
	assert.Equal(t, "CBB STP download failed", capturedSubject)
	assert.Contains(t, capturedBody, "https://skia-temporal-ui.corp.goog/namespaces/perf-internal/workflows/")
	assert.Contains(t, capturedBody, "missing &lt;dmg&gt; link &amp; error")
	assert.Contains(t, capturedBody, "https://developer.apple.com/safari/resources/")
	assert.Contains(t, capturedBody, "https://chrome-infra-packages.appspot.com/p/infra/chromeperf/cbb/safari_technology_preview")
	assert.Contains(t, capturedBody, "cipd set-ref")
	assert.Contains(t, capturedBody, "cipd create")
	env.AssertExpectations(t)
}

func TestSendCbbStpDownloadErrorEmail_DevTaskQueue_UsesExperimentalCipdPath(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var capturedBody string
	env.OnActivity(SendEmailActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedBody = args.Get(3).(string)
	}).Return(nil).Once()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{TaskQueue: "localhost.dev"})
		sendCbbStpDownloadErrorEmail(ctx, errors.New("dev run failure"))
		return nil
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Contains(t, capturedBody, "https://temporal-ui-dev.corp.goog/namespaces/perf-internal/workflows/")
	assert.Contains(t, capturedBody, "https://chrome-infra-packages.appspot.com/p/experimental/chromeperf/cbb/safari_technology_preview")
	assert.Contains(t, capturedBody, "cipd set-ref experimental/chromeperf/cbb/safari_technology_preview")
	assert.Contains(t, capturedBody, "cipd create -name experimental/chromeperf/cbb/safari_technology_preview")
	assert.NotContains(t, capturedBody, "infra/chromeperf/cbb/safari_technology_preview")
	env.AssertExpectations(t)
}

func TestSendCbbRunnerErrorEmail_SendsToChromeCbb(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var capturedTo []string
	var capturedSubject string
	env.OnActivity(SendEmailActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedTo = args.Get(1).([]string)
		capturedSubject = args.Get(2).(string)
	}).Return(nil).Once()

	p := &CbbRunnerParams{
		BotConfig: "mac-m4-mini-perf-cbb",
		Commit:    common.NewCombinedCommit(common.NewChromiumCommit("ee7ef8016281439932a81bdac80a66f9a31efb62")),
		Browser:   "chrome",
		Channel:   "stable",
	}
	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		sendCbbRunnerErrorEmail(ctx, "cbb_runner-chrome-stable-m4", p, errors.New("benchmark failed"))
		return nil
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, []string{"chrome-cbb@google.com"}, capturedTo)
	assert.Equal(t, `CBB runner "cbb_runner-chrome-stable-m4" failed`, capturedSubject)
	env.AssertExpectations(t)
}

func TestSendCbbBrowserVersionsErrorEmail_Edge_OmitsStpGuidance(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var capturedTo []string
	var capturedSubject, capturedBody string
	env.OnActivity(SendEmailActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedTo = args.Get(1).([]string)
		capturedSubject = args.Get(2).(string)
		capturedBody = args.Get(3).(string)
	}).Return(nil).Once()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		sendCbbBrowserVersionsErrorEmail(ctx, "edge", errors.New("browser version conflict: edge stable has both 130.0.2849.46 and 130.0.2849.52"))
		return nil
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, []string{errorNotificationEmail}, capturedTo)
	assert.Equal(t, "CBB edge version detection failed", capturedSubject)
	assert.Contains(t, capturedBody, "browser version conflict: edge stable has both 130.0.2849.46 and 130.0.2849.52")
	assert.Contains(t, capturedBody, "win-victus-perf-cbb")
	assert.Contains(t, capturedBody, "win-arm64-snapdragon-elite-perf-cbb")
	assert.NotContains(t, capturedBody, "safari_technology_preview")
	assert.NotContains(t, capturedBody, "cipd ")
	assert.NotContains(t, capturedBody, "developer.apple.com")
	env.AssertExpectations(t)
}

func TestSendCbbBrowserVersionsErrorEmail_DispatchesDiagnosticAlertEmail(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var capturedTo []string
	var capturedSubject, capturedBody string
	env.OnActivity(SendEmailActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedTo = args.Get(1).([]string)
		capturedSubject = args.Get(2).(string)
		capturedBody = args.Get(3).(string)
	}).Return(nil).Once()

	env.ExecuteWorkflow(func(ctx workflow.Context) error {
		sendCbbBrowserVersionsErrorEmail(ctx, "safari", errors.New("browser_versions task 123 finished with status BENCHMARK_FAILURE and no CAS output"))
		return nil
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, []string{errorNotificationEmail}, capturedTo)
	assert.Equal(t, "CBB safari version detection failed", capturedSubject)
	assert.Contains(t, capturedBody, "safari")
	assert.Contains(t, capturedBody, "https://skia-temporal-ui.corp.goog/namespaces/perf-internal/workflows/")
	assert.Contains(t, capturedBody, "browser_versions task 123 finished with status BENCHMARK_FAILURE and no CAS output")
	assert.Contains(t, capturedBody, "https://developer.apple.com/safari/resources/")
	assert.Contains(t, capturedBody, "https://chrome-infra-packages.appspot.com/p/infra/chromeperf/cbb/safari_technology_preview")
	assert.Contains(t, capturedBody, "cipd set-ref")
	assert.Contains(t, capturedBody, "cipd create")
	env.AssertExpectations(t)
}

func TestCbbNewReleaseDetectorWorkflow_DownloadSafariTPFails_SendsAlertEmailAndReturnsError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(CbbGetBrowserVersionsWorkflow, workflow.RegisterOptions{Name: workflows.CbbGetBrowserVersions})
	env.OnWorkflow(workflows.CbbGetBrowserVersions, mock.Anything, "safari").Return([]BuildInfo{}, nil).Once()
	env.OnWorkflow(workflows.CbbGetBrowserVersions, mock.Anything, "edge").Return([]BuildInfo{}, nil).Once()
	env.OnActivity(GetChromeReleasesInfoActivity, mock.Anything, mock.Anything, false).Return(&ChromeReleaseInfo{
		CommitPosition: "1704567",
		CommitHash:     "ee7ef8016281439932a81bdac80a66f9a31efb62",
		Builds:         nil,
	}, nil).Once()
	env.OnActivity(DownloadSafariTPActivity, mock.Anything, false).Return("", errors.New("unable to find STP download link for detected OS (macos27)")).Once()

	var capturedTo []string
	var capturedSubject, capturedBody string
	env.OnActivity(SendEmailActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedTo = args.Get(1).([]string)
		capturedSubject = args.Get(2).(string)
		capturedBody = args.Get(3).(string)
	}).Return(nil).Once()

	env.ExecuteWorkflow(CbbNewReleaseDetectorWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	assert.Contains(t, env.GetWorkflowError().Error(), "unable to find STP download link for detected OS (macos27)")
	assert.Equal(t, []string{errorNotificationEmail}, capturedTo)
	assert.Equal(t, "CBB STP download failed", capturedSubject)
	assert.Contains(t, capturedBody, "unable to find STP download link for detected OS (macos27)")
	assert.Contains(t, capturedBody, "https://developer.apple.com/safari/resources/")
	assert.Contains(t, capturedBody, "infra/chromeperf/cbb/safari_technology_preview")
	env.AssertExpectations(t)
}

func TestCbbNewReleaseDetectorWorkflow_GetBrowserVersionsFails_SendsAlertEmailAndContinues(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	edgeBuild := BuildInfo{
		Browser:  "edge",
		Channel:  "stable",
		Platform: "windows",
		Version:  "130.0.2849.46",
	}

	env.RegisterWorkflowWithOptions(CbbGetBrowserVersionsWorkflow, workflow.RegisterOptions{Name: workflows.CbbGetBrowserVersions})
	env.OnWorkflow(workflows.CbbGetBrowserVersions, mock.Anything, "safari").Return(nil, errors.New("browser_versions task failed on mac-m4-mini-perf-cbb")).Once()
	env.OnWorkflow(workflows.CbbGetBrowserVersions, mock.Anything, "edge").Return([]BuildInfo{edgeBuild}, nil).Once()

	var capturedTo []string
	var capturedSubject, capturedBody string
	env.OnActivity(SendEmailActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedTo = args.Get(1).([]string)
		capturedSubject = args.Get(2).(string)
		capturedBody = args.Get(3).(string)
	}).Return(nil).Once()

	env.OnActivity(GetChromeReleasesInfoActivity, mock.Anything, []BuildInfo{edgeBuild}, false).Return(&ChromeReleaseInfo{
		CommitPosition: "1704567",
		CommitHash:     "ee7ef8016281439932a81bdac80a66f9a31efb62",
		Builds:         nil,
	}, nil).Once()
	env.OnActivity(DownloadSafariTPActivity, mock.Anything, false).Return("254", nil).Once()

	env.ExecuteWorkflow(CbbNewReleaseDetectorWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	assert.Equal(t, []string{errorNotificationEmail}, capturedTo)
	assert.Equal(t, "CBB safari version detection failed", capturedSubject)
	assert.Contains(t, capturedBody, "browser_versions task failed on mac-m4-mini-perf-cbb")
	env.AssertExpectations(t)
}
