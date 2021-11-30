package poller

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	caches_mocks "go.skia.org/infra/skcq/go/caches/mocks"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
	"go.skia.org/infra/skcq/go/config"
	cfg_mocks "go.skia.org/infra/skcq/go/config/mocks"
	"go.skia.org/infra/skcq/go/db"
	db_mocks "go.skia.org/infra/skcq/go/db/mocks"
	"go.skia.org/infra/skcq/go/types"
	types_mocks "go.skia.org/infra/skcq/go/types/mocks"
)

func testProcessCL(t *testing.T, testVerifierStatuses []*types.VerifierStatus, expectedOverallState types.VerifierState, dryRun bool) {
	currentChangesCache := map[string]*types.CurrentlyProcessingChange{}
	httpClient := httputils.NewTimeoutClient()
	publicFEInstanceURL := "https://public-fe-url/"
	corpFEInstanceURL := "https://corp-fe-url/"
	ci := &gerrit.ChangeInfo{
		Issue:   int64(123),
		Status:  gerrit.ChangeStatusOpen,
		Subject: "Test change",
		Owner: &gerrit.Person{
			Email: "batman@gotham.com",
		},
		Project: "skia",
		Branch:  "main",
	}
	clsInThisRound := map[string]bool{}
	startTime := int64(111)
	changePatchsetID := "123/5"

	// Mock db.
	dbClient := &db_mocks.DB{}
	dbClient.On("PutChangeAttempt", testutils.AnyContext, mock.AnythingOfType("*types.ChangeAttempt"), db.GetChangesCol(false)).Return(nil).Once()

	// Mock current changes cache.
	cc := &caches_mocks.CurrentChangesCache{}
	cc.On("Get", testutils.AnyContext, dbClient).Return(currentChangesCache).Once()
	cc.On("Add", testutils.AnyContext, changePatchsetID, ci.Subject, "batman@gotham.com", ci.Project, ci.Branch, dryRun, false, ci.Issue, int64(5)).Return(startTime, false, nil).Once()
	cc.On("Remove", testutils.AnyContext, changePatchsetID).Return(nil).Once()

	// Mock cfg reader.
	skcfg := &config.SkCQCfg{}
	cfgReader := &cfg_mocks.ConfigReader{}
	cfgReader.On("GetSkCQCfg", testutils.AnyContext).Return(skcfg, nil).Once()

	// Mock codereview.
	cr := &cr_mocks.CodeReview{}
	cr.On("IsDryRun", testutils.AnyContext, ci).Return(dryRun).Once()
	cr.On("IsCQ", testutils.AnyContext, ci).Return(!dryRun)
	cr.On("GetEarliestEquivalentPatchSetID", ci).Return(int64(5)).Once()
	cr.On("GetLatestPatchSetID", ci).Return(int64(5)).Twice()
	if expectedOverallState != types.VerifierWaitingState {
		cr.On("RemoveFromCQ", testutils.AnyContext, ci, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Once()
	}
	if !dryRun && expectedOverallState == types.VerifierSuccessState {
		cr.On("Submit", testutils.AnyContext, ci).Return(nil).Once()
	}

	// Mock verifier manager.
	vm := &types_mocks.VerifiersManager{}
	vm.On("GetVerifiers", testutils.AnyContext, skcfg, ci, false, cfgReader).Return([]types.Verifier{}, []string{}, nil).Once()
	vm.On("RunVerifiers", testutils.AnyContext, ci, []types.Verifier{}, startTime).Return(testVerifierStatuses).Once()

	// Mock throttler maanger.
	tm := &types_mocks.ThrottlerManager{}
	tm.On("UpdateThrottler", "skia/main", mock.AnythingOfType("time.Time"), skcfg.ThrottlerCfg).Once()

	processCL(context.Background(), vm, ci, cfgReader, clsInThisRound, cr, cc, httpClient, dbClient, nil, publicFEInstanceURL, corpFEInstanceURL, tm)
}

func TestProcessCL_DryRun_FailureOverallState(t *testing.T) {
	unittest.SmallTest(t)

	testVerifierStatuses := []*types.VerifierStatus{
		{State: types.VerifierSuccessState, Name: "Verifier1", Reason: "Reason1"},
		{State: types.VerifierFailureState, Name: "Verifier2", Reason: "Reason2"},
		{State: types.VerifierWaitingState, Name: "Verifier3", Reason: "Reason3"},
	}
	testProcessCL(t, testVerifierStatuses, types.VerifierFailureState, true)
}

func TestProcessCL_DryRun_SuccessOverallState(t *testing.T) {
	unittest.SmallTest(t)

	testVerifierStatuses := []*types.VerifierStatus{
		{State: types.VerifierSuccessState, Name: "Verifier1", Reason: "Reason1"},
		{State: types.VerifierSuccessState, Name: "Verifier2", Reason: "Reason2"},
	}
	testProcessCL(t, testVerifierStatuses, types.VerifierSuccessState, true)
}

func TestProcessCL_DryRun_WaitingOverallState(t *testing.T) {
	unittest.SmallTest(t)

	testVerifierStatuses := []*types.VerifierStatus{
		{State: types.VerifierSuccessState, Name: "Verifier1", Reason: "Reason1"},
		{State: types.VerifierWaitingState, Name: "Verifier2", Reason: "Reason2"},
		{State: types.VerifierSuccessState, Name: "Verifier3", Reason: "Reason3"},
	}
	testProcessCL(t, testVerifierStatuses, types.VerifierSuccessState, true)
}

func TestProcessCL_CQRun_FailureOverallState(t *testing.T) {
	unittest.SmallTest(t)

	testVerifierStatuses := []*types.VerifierStatus{
		{State: types.VerifierSuccessState, Name: "Verifier1", Reason: "Reason1"},
		{State: types.VerifierFailureState, Name: "Verifier2", Reason: "Reason2"},
		{State: types.VerifierWaitingState, Name: "Verifier3", Reason: "Reason3"},
	}
	testProcessCL(t, testVerifierStatuses, types.VerifierFailureState, false)
}

func TestProcessCL_CQRun_WaitingOverallState(t *testing.T) {
	unittest.SmallTest(t)

	testVerifierStatuses := []*types.VerifierStatus{
		{State: types.VerifierSuccessState, Name: "Verifier1", Reason: "Reason1"},
		{State: types.VerifierWaitingState, Name: "Verifier2", Reason: "Reason2"},
	}
	testProcessCL(t, testVerifierStatuses, types.VerifierWaitingState, false)
}

func TestProcessCL_CQRun_SuccessOverallState(t *testing.T) {
	unittest.SmallTest(t)

	testVerifierStatuses := []*types.VerifierStatus{
		{State: types.VerifierSuccessState, Name: "Verifier1", Reason: "Reason1"},
		{State: types.VerifierSuccessState, Name: "Verifier2", Reason: "Reason2"},
	}
	testProcessCL(t, testVerifierStatuses, types.VerifierSuccessState, false)
}

func TestCleanupCL(t *testing.T) {
	unittest.SmallTest(t)

	changeID := int64(123)
	equivalentPatchsetID := int64(5)
	changePatchsetID := fmt.Sprintf("%d/%d", changeID, equivalentPatchsetID)
	latestPatchsetID := int64(7)
	httpClient := httputils.NewTimeoutClient()
	ci := &gerrit.ChangeInfo{
		Issue: int64(123),
	}
	startTime := int64(444)
	cqRecord := &types.CurrentlyProcessingChange{
		ChangeID:         int64(123),
		LatestPatchsetID: latestPatchsetID,
		StartTs:          startTime,
	}

	// Mock current changes cache.
	cc := &caches_mocks.CurrentChangesCache{}
	cc.On("Remove", testutils.AnyContext, changePatchsetID).Return(nil).Once()

	// Mock db.
	dbClient := &db_mocks.DB{}
	dbClient.On("UpdateChangeAttemptAsAbandoned", testutils.AnyContext, changeID, latestPatchsetID, db.GetChangesCol(false), startTime).Return(nil).Once()

	// Mock cfg reader.
	skcfg := &config.SkCQCfg{}
	cfgReader := &cfg_mocks.ConfigReader{}
	cfgReader.On("GetSkCQCfg", testutils.AnyContext).Return(skcfg, nil).Once()

	// Mock codereview.
	cr := &cr_mocks.CodeReview{}

	// Mock 2 verifiers.
	v1 := &types_mocks.Verifier{}
	v1.On("Cleanup", testutils.AnyContext, ci, equivalentPatchsetID)
	v2 := &types_mocks.Verifier{}
	v2.On("Cleanup", testutils.AnyContext, ci, equivalentPatchsetID)

	// Mock verifier manager.
	vm := &types_mocks.VerifiersManager{}
	vm.On("GetVerifiers", testutils.AnyContext, skcfg, ci, false, cfgReader).Return([]types.Verifier{v1, v2}, []string{}, nil).Once()

	err := cleanupCL(context.Background(), changePatchsetID, cc, dbClient, cqRecord, ci, cfgReader, cr, httpClient, vm)
	require.Nil(t, err)
}
