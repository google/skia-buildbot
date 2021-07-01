package poller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	caches_mocks "go.skia.org/infra/skcq/go/caches/mocks"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
	"go.skia.org/infra/skcq/go/config"
	cfg_mocks "go.skia.org/infra/skcq/go/config/mocks"
	db_mocks "go.skia.org/infra/skcq/go/db/mocks"
	"go.skia.org/infra/skcq/go/types"
	types_mocks "go.skia.org/infra/skcq/go/types/mocks"
)

func TestProcessCL(t *testing.T) {
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
	dbClient.On("PutChangeAttempt", testutils.AnyContext, mock.AnythingOfType("*types.ChangeAttempt"), false).Return(nil).Once()

	// Mock current changes cache.
	cc := &caches_mocks.CurrentChangesCache{}
	cc.On("Get", testutils.AnyContext, dbClient).Return(currentChangesCache).Once()
	cc.On("Add", testutils.AnyContext, changePatchsetID, ci.Subject, "batman@gotham.com", ci.Project, ci.Branch, true, false, ci.Issue, int64(5)).Return(startTime, false, nil).Once()
	cc.On("Remove", testutils.AnyContext, changePatchsetID).Return(nil).Once()

	// Mock cfg reader.
	skcfg := &config.SkCQCfg{}
	cfgReader := &cfg_mocks.ConfigReader{}
	cfgReader.On("GetSkCQCfg", testutils.AnyContext).Return(skcfg, nil).Once()

	// Mock codereview.
	cr := &cr_mocks.CodeReview{}
	// cr.On("GetIssueProperties", testutils.AnyContext, ci.Issue).Return(ci, nil).Once()
	cr.On("IsDryRun", testutils.AnyContext, ci).Return(true).Times(3)
	cr.On("IsCQ", testutils.AnyContext, ci).Return(false).Once()
	cr.On("GetEarliestEquivalentPatchSetID", ci).Return(int64(5)).Once()
	cr.On("GetLatestPatchSetID", ci).Return(int64(5)).Twice()
	cr.On("RemoveFromCQ", testutils.AnyContext, ci, mock.AnythingOfType("string")).Once()
	// cr.On("GetFileNames", testutils.AnyContext, ci).Return([]string{"dir1/file1"}]).Once()

	// Mock verifier manager.
	vm := &types_mocks.VerifiersManager{}
	vm.On("GetVerifiers", testutils.AnyContext, httpClient, skcfg, cr, ci, false, cfgReader).Return([]types.Verifier{}, []string{}, nil).Once()
	verifierStatuses := []*types.VerifierStatus{
		{State: types.VerifierSuccessState, Name: "Verifier1", Reason: "Reason1"},
		{State: types.VerifierFailureState, Name: "Verifier2", Reason: "Reason2"},
	}
	vm.On("RunVerifiers", testutils.AnyContext, ci, []types.Verifier{}, startTime).Return(verifierStatuses).Once()

	processCL(context.Background(), vm, ci, cfgReader, clsInThisRound, cr, cc, httpClient, dbClient, nil, publicFEInstanceURL, corpFEInstanceURL)

	// Put this in setup and send in verifiers and their statuses??
}
