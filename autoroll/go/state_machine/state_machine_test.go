package state_machine

import (
	"context"
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/testutils"
)

const (
	gcsPrefix = "test-roller"
)

// A completely mocked implementation of RollCLImpl.
type TestRollCLImpl struct {
	t            *testing.T
	closedStatus string
	closedMsg    string
	isDryRun     bool
	normalResult string
	dryRunResult string
	rollingTo    string
}

// Return a TestRollCLImpl instance.
func NewTestRollCLImpl(t *testing.T, rollingTo string, isDryRun bool) *TestRollCLImpl {
	return &TestRollCLImpl{
		t:         t,
		isDryRun:  isDryRun,
		rollingTo: rollingTo,
	}
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) AddComment(string) error {
	return nil
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) Close(ctx context.Context, status, msg string) error {
	r.closedStatus = status
	r.closedMsg = msg
	return nil
}

// Assert that we closed the CL with the given status.
func (r *TestRollCLImpl) AssertClosed(status string) {
	assert.Equal(r.t, status, r.closedStatus)
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) IsFinished() bool {
	return r.normalResult != ""
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) IsSuccess() bool {
	return r.normalResult == "SUCCESS"
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) IsDryRunFinished() bool {
	return r.dryRunResult != ""
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) IsDryRunSuccess() bool {
	return r.dryRunResult == "SUCCESS"
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) IssueID() string {
	return "0"
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) IssueURL() string {
	return "http://codereview/123"
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) RollingTo() string {
	return r.rollingTo
}

// Mark the roll as a success.
func (r *TestRollCLImpl) SetSucceeded() {
	r.normalResult = "SUCCESS"
}

// Mark the roll as a failure.
func (r *TestRollCLImpl) SetFailed() {
	r.normalResult = "FAILED"
}

// Mark the roll as a success.
func (r *TestRollCLImpl) SetDryRunSucceeded() {
	r.dryRunResult = "SUCCESS"
}

// Mark the roll as a failure.
func (r *TestRollCLImpl) SetDryRunFailed() {
	r.dryRunResult = "FAILED"
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) SwitchToDryRun(ctx context.Context) error {
	r.isDryRun = true
	return nil
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) SwitchToNormal(ctx context.Context) error {
	r.isDryRun = false
	return nil
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) RetryDryRun(ctx context.Context) error {
	r.isDryRun = true
	r.dryRunResult = ""
	return nil
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) RetryCQ(ctx context.Context) error {
	r.isDryRun = false
	r.normalResult = ""
	return nil
}

// Assert that the roll is in dry run mode.
func (r *TestRollCLImpl) AssertDryRun() {
	assert.True(r.t, r.isDryRun)
}

// Assert that the roll is not in dry run mode.
func (r *TestRollCLImpl) AssertNotDryRun() {
	assert.False(r.t, r.isDryRun)
}

// See documentation for RollCLImpl.
func (r *TestRollCLImpl) Update(ctx context.Context) error {
	return nil
}

// A completely mocked implementation of AutoRollerImpl. Allows setting results
// for every member function.
type TestAutoRollerImpl struct {
	t *testing.T

	createNewRollResult *TestRollCLImpl
	createNewRollError  error

	failureThrottle *Throttler

	getActiveRollResult *TestRollCLImpl

	getCurrentRevResult string
	getCurrentRevError  error

	getNextRollRevResult string
	getNextRollRevError  error

	getModeResult   string
	rolledPast      map[string]bool
	safetyThrottle  *Throttler
	successThrottle *Throttler
	updateError     error
}

// Return a TestAutoRollerImpl instance.
func NewTestAutoRollerImpl(t *testing.T, ctx context.Context, gcsClient gcs.GCSClient) *TestAutoRollerImpl {
	failureThrottle, err := NewThrottler(ctx, gcsClient, "", time.Duration(0), 0)
	assert.NoError(t, err)
	safetyThrottle, err := NewThrottler(ctx, gcsClient, "", time.Duration(0), 0)
	assert.NoError(t, err)
	successThrottle, err := NewThrottler(ctx, gcsClient, "", time.Duration(0), 0)
	assert.NoError(t, err)
	return &TestAutoRollerImpl{
		t:               t,
		failureThrottle: failureThrottle,
		getModeResult:   modes.MODE_RUNNING,
		rolledPast:      map[string]bool{},
		safetyThrottle:  safetyThrottle,
		successThrottle: successThrottle,
	}
}

// See documentation for AutoRollerImpl.
func (r *TestAutoRollerImpl) UploadNewRoll(ctx context.Context, from, to string, dryRun bool) error {
	if r.createNewRollError != nil {
		return r.createNewRollError
	}
	r.getActiveRollResult = r.createNewRollResult
	if r.getActiveRollResult == nil {
		r.getActiveRollResult = NewTestRollCLImpl(r.t, to, dryRun)
	}
	r.createNewRollResult = nil
	return nil
}

// Set the result of UploadNewRoll. If this is not called before UploadNewRoll,
// an empty TestRollCLImpl will be used.
func (r *TestAutoRollerImpl) SetUploadNewRollResult(roll *TestRollCLImpl, err error) {
	r.createNewRollResult = roll
	r.createNewRollError = err
}

// See documentation for AutoRollerImpl.
func (r *TestAutoRollerImpl) GetActiveRoll() RollCLImpl {
	return r.getActiveRollResult
}

// Set the result of GetActiveRoll. Note that the active roll is automatically
// set during UploadNewRoll so this should only be needed for initial setup or
// when setting an error to be returned.
func (r *TestAutoRollerImpl) SetActiveRoll(roll *TestRollCLImpl) {
	r.getActiveRollResult = roll
}

// See documentation for AutoRollerImpl.
func (r *TestAutoRollerImpl) GetCurrentRev() string {
	return r.getCurrentRevResult
}

// Set the result of GetCurrentRev.
func (r *TestAutoRollerImpl) SetCurrentRev(rev string) {
	r.getCurrentRevResult = rev
}

// See documentation for AutoRollerImpl.
func (r *TestAutoRollerImpl) GetNextRollRev() string {
	return r.getNextRollRevResult
}

// Set the result of GetNextRollRev.
func (r *TestAutoRollerImpl) SetNextRollRev(rev string) {
	r.getNextRollRevResult = rev
}

// See documentation for AutoRollerImpl.
func (r *TestAutoRollerImpl) GetMode() string {
	return r.getModeResult
}

// Set the result of GetMode.
func (r *TestAutoRollerImpl) SetMode(ctx context.Context, mode string) {
	r.getModeResult = mode
}

// See documentation for AutoRollerImpl.
func (r *TestAutoRollerImpl) RolledPast(ctx context.Context, rev string) (bool, error) {
	rv, ok := r.rolledPast[rev]
	assert.True(r.t, ok)
	delete(r.rolledPast, rev)
	return rv, nil
}

// Set the result of RolledPast.
func (r *TestAutoRollerImpl) SetRolledPast(rev string, result bool) {
	_, ok := r.rolledPast[rev]
	assert.False(r.t, ok)
	r.rolledPast[rev] = result
}

// See documentation for AutoRollerImpl.
func (r *TestAutoRollerImpl) UpdateRepos(ctx context.Context) error {
	return r.updateError
}

// Set the error returned from Update.
func (r *TestAutoRollerImpl) SetUpdateError(err error) {
	r.updateError = err
}

// Return a Throttler indicating that we have failed to roll too many
// times within a time period.
func (r *TestAutoRollerImpl) FailureThrottle() *Throttler {
	return r.failureThrottle
}

// Return a Throttler indicating that we have attempted to upload too
// many CLs within a time period.
func (r *TestAutoRollerImpl) SafetyThrottle() *Throttler {
	return r.safetyThrottle
}

// Return a Throttler indicating whether we have successfully rolled too
// many times within a time period.
func (r *TestAutoRollerImpl) SuccessThrottle() *Throttler {
	return r.successThrottle
}

// Assert that the StateMachine is in the given state.
func checkState(t *testing.T, sm *AutoRollStateMachine, wanted string) {
	assert.Equal(t, wanted, sm.Current())
}

// Perform a state transition and assert that we ended up in the given state.
func checkNextState(t *testing.T, sm *AutoRollStateMachine, wanted string) {
	assert.NoError(t, sm.NextTransition(context.Background()))
	assert.Equal(t, wanted, sm.Current())
}

// Shared setup.
func setup(t *testing.T) (context.Context, *AutoRollStateMachine, *TestAutoRollerImpl, gcs.GCSClient, func()) {
	testutils.MediumTest(t)

	ctx := context.Background()
	gcsClient := gcs.NewMemoryGCSClient("test-bucket")
	rollerImpl := NewTestAutoRollerImpl(t, ctx, gcsClient)
	n, err := notifier.New(ctx, "fake", "fake", "fake", nil, nil, nil)
	assert.NoError(t, err)
	sm, err := New(ctx, rollerImpl, n, gcsClient, "test-roller")
	assert.NoError(t, err)
	return ctx, sm, rollerImpl, gcsClient, func() {}
}

func TestNormal(t *testing.T) {
	ctx, sm, r, gcsClient, cleanup := setup(t)
	defer cleanup()

	failureThrottle, err := NewThrottler(ctx, gcsClient, "fail_counter", time.Hour, 1)
	assert.NoError(t, err)
	r.failureThrottle = failureThrottle

	checkState(t, sm, S_NORMAL_IDLE)

	// Ensure that we stay idle.
	checkNextState(t, sm, S_NORMAL_IDLE)

	// Create a new roll.
	r.SetNextRollRev("HEAD+1")
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll := r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertNotDryRun()

	// Still active.
	checkNextState(t, sm, S_NORMAL_ACTIVE)

	// Roll finished successfully.
	roll.SetSucceeded()
	rev := r.GetNextRollRev()
	r.SetCurrentRev(rev)
	checkNextState(t, sm, S_NORMAL_SUCCESS)
	r.SetRolledPast("HEAD+1", true)
	checkNextState(t, sm, S_NORMAL_IDLE)

	// Ensure that we stay idle.
	checkNextState(t, sm, S_NORMAL_IDLE)

	// Create a new roll.
	r.SetNextRollRev("HEAD+2")
	checkNextState(t, sm, S_NORMAL_ACTIVE)

	// This one failed, but there's no new commit. Ensure that we retry
	// the active CL after a wait.
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertNotDryRun()
	roll.SetFailed()
	checkNextState(t, sm, S_NORMAL_FAILURE)
	checkNextState(t, sm, S_NORMAL_FAILURE_THROTTLED)
	// Should stay throttled.
	checkNextState(t, sm, S_NORMAL_FAILURE_THROTTLED)
	// We still have to respect mode changes.
	r.SetMode(ctx, modes.MODE_DRY_RUN)
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertDryRun()
	r.SetMode(ctx, modes.MODE_RUNNING)
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll.SetFailed()
	checkNextState(t, sm, S_NORMAL_FAILURE)
	checkNextState(t, sm, S_NORMAL_FAILURE_THROTTLED)
	// Someone might forcibly submit the CL while we're throttled.
	roll.SetSucceeded()
	r.SetRolledPast("HEAD+2", true)
	r.SetNextRollRev("HEAD+3")
	checkNextState(t, sm, S_NORMAL_SUCCESS)
	checkNextState(t, sm, S_NORMAL_IDLE)
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.SetFailed()
	checkNextState(t, sm, S_NORMAL_FAILURE)
	checkNextState(t, sm, S_NORMAL_FAILURE_THROTTLED)

	// Hack the timer to fake that the throttling has expired, then ensure
	// that we retry the CQ.
	counterFile := "fail_counter"
	assert.NoError(t, gcsClient.DeleteFile(ctx, counterFile))
	failureThrottle, err = NewThrottler(ctx, gcsClient, counterFile, time.Minute, 1)
	assert.NoError(t, err)
	r.failureThrottle = failureThrottle
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	uploaded := r.GetActiveRoll()
	assert.Equal(t, roll, uploaded)

	// Failed again, and now there's a new commit. Ensure that we exit
	// failure-throttling, close the active CL and upload another.
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertNotDryRun()
	roll.SetFailed()
	checkNextState(t, sm, S_NORMAL_FAILURE)
	checkNextState(t, sm, S_NORMAL_FAILURE_THROTTLED)
	r.SetNextRollRev("HEAD+4")
	checkNextState(t, sm, S_NORMAL_IDLE)
	roll.AssertClosed(autoroll.ROLL_RESULT_FAILURE)
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	uploaded = r.GetActiveRoll()
	assert.NotEqual(t, roll, uploaded)

	// Failed yet again, with yet another new commit. Ensure that we close
	// the active CL and upload another, without failure-throttling.
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.SetFailed()
	r.SetNextRollRev("HEAD+5")
	checkNextState(t, sm, S_NORMAL_FAILURE)
	checkNextState(t, sm, S_NORMAL_IDLE)
	roll.AssertClosed(autoroll.ROLL_RESULT_FAILURE)
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	uploaded = r.GetActiveRoll()
	assert.NotEqual(t, roll, uploaded)

	// Roll succeeded.
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertNotDryRun()
	roll.SetSucceeded()
	r.SetCurrentRev(r.GetNextRollRev())
	checkNextState(t, sm, S_NORMAL_SUCCESS)
	r.SetRolledPast("HEAD+5", true)
	checkNextState(t, sm, S_NORMAL_IDLE)

	// Upload a new roll, which fails. Ensure that we can stop the roller
	// from the throttled state.
	r.SetNextRollRev("HEAD+6")
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.SetFailed()
	checkNextState(t, sm, S_NORMAL_FAILURE)
	checkNextState(t, sm, S_NORMAL_FAILURE_THROTTLED)
	r.SetMode(ctx, modes.MODE_STOPPED)
	checkNextState(t, sm, S_STOPPED)
	roll.AssertClosed(autoroll.ROLL_RESULT_FAILURE)
	// We don't reopen the CL and go back to throttled state in this case.
	r.SetMode(ctx, modes.MODE_RUNNING)
	checkNextState(t, sm, S_NORMAL_IDLE)
}

func TestDryRun(t *testing.T) {
	ctx, sm, r, gcsClient, cleanup := setup(t)
	defer cleanup()

	failureThrottle, err := NewThrottler(ctx, gcsClient, "fail_counter", time.Hour, 1)
	assert.NoError(t, err)
	r.failureThrottle = failureThrottle

	// Switch to dry run.
	checkState(t, sm, S_NORMAL_IDLE)
	r.SetMode(ctx, modes.MODE_DRY_RUN)
	checkNextState(t, sm, S_DRY_RUN_IDLE)

	// Create a new roll.
	r.SetNextRollRev("HEAD+1")
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)

	// Still active.
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	roll := r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertDryRun()

	// Roll finished successfully, leave it open for a bit.
	roll.SetDryRunSucceeded()
	checkNextState(t, sm, S_DRY_RUN_SUCCESS)
	checkNextState(t, sm, S_DRY_RUN_SUCCESS_LEAVING_OPEN)
	checkNextState(t, sm, S_DRY_RUN_SUCCESS_LEAVING_OPEN)
	checkNextState(t, sm, S_DRY_RUN_SUCCESS_LEAVING_OPEN)

	// A new commit landed. Assert that we closed the CL, upload another.
	r.SetNextRollRev("HEAD+2")
	checkNextState(t, sm, S_DRY_RUN_IDLE)
	roll.AssertClosed(autoroll.ROLL_RESULT_DRY_RUN_SUCCESS)
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)

	// This one failed, but there's no new commit. Ensure that we retry
	// the active CL after a wait.
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertDryRun()
	roll.SetDryRunFailed()
	checkNextState(t, sm, S_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_FAILURE_THROTTLED)
	// Should stay throttled.
	checkNextState(t, sm, S_DRY_RUN_FAILURE_THROTTLED)
	// We still have to respect mode changes.
	r.SetMode(ctx, modes.MODE_RUNNING)
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertNotDryRun()
	r.SetMode(ctx, modes.MODE_DRY_RUN)
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	roll.SetFailed()
	checkNextState(t, sm, S_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_FAILURE_THROTTLED)
	// Someone might forcibly submit the CL while we're throttled.
	roll.SetSucceeded()
	r.SetRolledPast("HEAD+2", true)
	r.SetNextRollRev("HEAD+3")
	checkNextState(t, sm, S_DRY_RUN_IDLE)
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.SetDryRunFailed()
	checkNextState(t, sm, S_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_FAILURE_THROTTLED)
	// Hack the timer to fake that the throttling has expired, then ensure
	// that we retry the CQ.
	counterFile := "fail_counter"
	assert.NoError(t, gcsClient.DeleteFile(ctx, counterFile))
	failureThrottle, err = NewThrottler(ctx, gcsClient, counterFile, time.Minute, 1)
	assert.NoError(t, err)
	r.failureThrottle = failureThrottle
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	uploaded := r.GetActiveRoll()
	assert.Equal(t, roll, uploaded)

	// Failed again, and now there's a new commit. Ensure that we exit
	// failure-throttling, close the active CL and upload another.
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertDryRun()
	roll.SetDryRunFailed()
	checkNextState(t, sm, S_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_FAILURE_THROTTLED)
	r.SetNextRollRev("HEAD+4")
	checkNextState(t, sm, S_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_IDLE)
	roll.AssertClosed(autoroll.ROLL_RESULT_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	uploaded = r.GetActiveRoll()
	assert.NotEqual(t, roll, uploaded)

	// Failed yet again, with yet another new commit. Ensure that we close
	// the active CL and upload another, without failure-throttling.
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	roll.SetDryRunFailed()
	r.SetNextRollRev("HEAD+5")
	checkNextState(t, sm, S_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_IDLE)
	roll.AssertClosed(autoroll.ROLL_RESULT_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	uploaded = r.GetActiveRoll()
	assert.NotEqual(t, roll, uploaded)

	// This one failed too. Ensure that we can stop the roller from the
	// throttled state.
	uploaded.(*TestRollCLImpl).SetDryRunFailed()
	checkNextState(t, sm, S_DRY_RUN_FAILURE)
	checkNextState(t, sm, S_DRY_RUN_FAILURE_THROTTLED)
	r.SetMode(ctx, modes.MODE_STOPPED)
	checkNextState(t, sm, S_STOPPED)
	roll.AssertClosed(autoroll.ROLL_RESULT_DRY_RUN_FAILURE)
	// We don't reopen the CL and go back to throttled state in this case.
	r.SetMode(ctx, modes.MODE_DRY_RUN)
	checkNextState(t, sm, S_DRY_RUN_IDLE)
}

func TestNormalToDryRun(t *testing.T) {
	ctx, sm, r, _, cleanup := setup(t)
	defer cleanup()

	// Upload a roll and switch it in and out of dry run mode.
	checkState(t, sm, S_NORMAL_IDLE)
	r.SetNextRollRev("HEAD+1")
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll := r.GetActiveRoll().(*TestRollCLImpl)
	roll.AssertNotDryRun()
	r.SetMode(ctx, modes.MODE_DRY_RUN)
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	roll.AssertDryRun()
	r.SetMode(ctx, modes.MODE_RUNNING)
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll.AssertNotDryRun()
}

func TestStopped(t *testing.T) {
	ctx, sm, r, _, cleanup := setup(t)
	defer cleanup()

	// Switch in and out of stopped mode.
	checkState(t, sm, S_NORMAL_IDLE)
	r.SetMode(ctx, modes.MODE_STOPPED)
	checkNextState(t, sm, S_STOPPED)
	checkNextState(t, sm, S_STOPPED)
	r.SetMode(ctx, modes.MODE_DRY_RUN)
	checkNextState(t, sm, S_DRY_RUN_IDLE)
	r.SetMode(ctx, modes.MODE_STOPPED)
	checkNextState(t, sm, S_STOPPED)

	r.SetNextRollRev("HEAD+1")
	r.SetMode(ctx, modes.MODE_RUNNING)
	checkNextState(t, sm, S_NORMAL_IDLE)
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll := r.GetActiveRoll().(*TestRollCLImpl)
	r.SetMode(ctx, modes.MODE_STOPPED)
	checkNextState(t, sm, S_STOPPED)
	roll.AssertClosed(autoroll.ROLL_RESULT_FAILURE)
	r.SetMode(ctx, modes.MODE_DRY_RUN)
	checkNextState(t, sm, S_DRY_RUN_IDLE)
	checkNextState(t, sm, S_DRY_RUN_ACTIVE)
	roll = r.GetActiveRoll().(*TestRollCLImpl)
	r.SetMode(ctx, modes.MODE_STOPPED)
	checkNextState(t, sm, S_STOPPED)
	roll.AssertClosed(autoroll.ROLL_RESULT_FAILURE)
}

func testSafetyThrottle(t *testing.T, mode string, attemptCount int64, period time.Duration) {
	ctx, sm, r, gcsClient, cleanup := setup(t)
	defer cleanup()

	safetyThrottle, err := NewThrottler(ctx, gcsClient, "attempt_counter", period, attemptCount)
	assert.NoError(t, err)
	r.safetyThrottle = safetyThrottle

	// Upload a bunch of CLs, fail fast until we're throttled.
	checkState(t, sm, S_NORMAL_IDLE)
	r.SetMode(ctx, mode)
	assert.NoError(t, sm.NextTransition(ctx))
	n := 1
	r.SetNextRollRev(fmt.Sprintf("HEAD+%d", n))
	for i := int64(0); i < attemptCount; i++ {
		assert.NoError(t, sm.NextTransition(ctx))
		roll := r.GetActiveRoll().(*TestRollCLImpl)
		n++
		r.SetNextRollRev(fmt.Sprintf("HEAD+%d", n))
		if mode == modes.MODE_DRY_RUN {
			roll.SetDryRunFailed()
		} else {
			roll.SetFailed()
		}
		assert.NoError(t, sm.NextTransition(ctx))
		assert.NoError(t, sm.NextTransition(ctx))
		if mode == modes.MODE_DRY_RUN {
			roll.AssertClosed(autoroll.ROLL_RESULT_DRY_RUN_FAILURE)
		} else {
			roll.AssertClosed(autoroll.ROLL_RESULT_FAILURE)
		}
	}
	// Now we should be throttled.
	throttled := S_NORMAL_SAFETY_THROTTLED
	if mode == modes.MODE_DRY_RUN {
		throttled = S_DRY_RUN_SAFETY_THROTTLED
	}
	checkNextState(t, sm, throttled)

	// Make sure we stay throttled.
	checkNextState(t, sm, throttled)

	// Make sure we get unthrottled when it's time.

	// Rather than waiting for the time window to pass, create a new
	// Throttler to fake it, assuming that the counter works as
	// it should.
	safetyThrottle, err = NewThrottler(ctx, gcsClient, "attempt_counter2", period, attemptCount)
	assert.NoError(t, err)
	r.safetyThrottle = safetyThrottle
	assert.Equal(t, throttled, sm.Current())
	idle := S_NORMAL_IDLE
	if mode == modes.MODE_DRY_RUN {
		idle = S_DRY_RUN_IDLE
	}
	checkNextState(t, sm, idle)
}

func TestSafetyThrottle(t *testing.T) {
	testSafetyThrottle(t, modes.MODE_RUNNING, 3, 30*time.Minute)
}

func TestSafetyThrottleDryRun(t *testing.T) {
	testSafetyThrottle(t, modes.MODE_DRY_RUN, 3, 30*time.Minute)
}

func TestPersistence(t *testing.T) {
	ctx, sm, r, gcsClient, cleanup := setup(t)
	defer cleanup()

	check := func() {
		n, err := notifier.New(ctx, "fake", "fake", "fake", nil, nil, nil)
		assert.NoError(t, err)
		sm2, err := New(ctx, r, n, gcsClient, gcsPrefix)
		assert.NoError(t, err)
		assert.Equal(t, sm.Current(), sm2.Current())
	}

	// Go through a series of transitions and verify that we get the same
	// state back from a duplicate state machine.
	check()
	r.SetNextRollRev("HEAD+1")
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	check()
	roll := r.GetActiveRoll().(*TestRollCLImpl)
	roll.SetFailed()
	checkNextState(t, sm, S_NORMAL_FAILURE)
	check()
	r.SetNextRollRev("HEAD+2")
	checkNextState(t, sm, S_NORMAL_IDLE)
	check()
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	check()
}

func TestSuccessThrottle(t *testing.T) {
	ctx, sm, r, gcsClient, cleanup := setup(t)
	defer cleanup()

	counterFile := "success_counter"
	successThrottle, err := NewThrottler(ctx, gcsClient, counterFile, 30*time.Minute, 1)
	assert.NoError(t, err)
	r.successThrottle = successThrottle

	checkNextState(t, sm, S_NORMAL_IDLE)
	r.SetNextRollRev("HEAD+1")
	checkNextState(t, sm, S_NORMAL_ACTIVE)
	roll := r.GetActiveRoll().(*TestRollCLImpl)
	roll.SetSucceeded()
	checkNextState(t, sm, S_NORMAL_SUCCESS)
	r.SetRolledPast("HEAD+1", true)

	// We should've incremented the counter, which should put us in the
	// SUCCESS_THROTTLED state.
	checkNextState(t, sm, S_NORMAL_SUCCESS_THROTTLED)
	checkNextState(t, sm, S_NORMAL_SUCCESS_THROTTLED)
	// We should still respect switches to dry run in this state.
	r.SetMode(ctx, modes.MODE_DRY_RUN)
	checkNextState(t, sm, S_DRY_RUN_IDLE)
	// And when we switch back we should still be throttled.
	r.SetMode(ctx, modes.MODE_RUNNING)
	checkNextState(t, sm, S_NORMAL_SUCCESS_THROTTLED)
	// Now, stop the roller, restart it, and ensure that we're still
	// throttled.
	r.SetMode(ctx, modes.MODE_STOPPED)
	checkNextState(t, sm, S_STOPPED)
	r.SetMode(ctx, modes.MODE_RUNNING)
	checkNextState(t, sm, S_NORMAL_IDLE)
	checkNextState(t, sm, S_NORMAL_SUCCESS_THROTTLED)
	// This would continue for the next 30 minutes... Instead, we'll hack
	// the counter to pretend it timed out.
	assert.NoError(t, gcsClient.DeleteFile(ctx, counterFile))
	successThrottle, err = NewThrottler(ctx, gcsClient, counterFile, time.Minute, 1)
	assert.NoError(t, err)
	r.successThrottle = successThrottle
	checkNextState(t, sm, S_NORMAL_IDLE)
}
