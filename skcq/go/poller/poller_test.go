package poller

import (
	"context"
	"testing"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	caches_mocks "go.skia.org/infra/skcq/go/caches/mocks"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
	"go.skia.org/infra/skcq/go/config"
	cfg_mocks "go.skia.org/infra/skcq/go/config/mocks"
	"go.skia.org/infra/skcq/go/db"
	"go.skia.org/infra/skcq/go/types"
)

func TestProcessCL(t *testing.T) {
	dbClient := &db.FirestoreDB{}
	currentChangesCache := map[string]*types.CurrentlyProcessingChange{}
	httpClient := httputils.NewTimeoutClient()
	publicFEInstanceURL := "https://public-fe-url/"
	corpFEInstanceURL := "https://corp-fe-url/"
	ci := &gerrit.ChangeInfo{
		Issue:  int64(123),
		Status: gerrit.ChangeStatusOpen,
	}
	clsInThisRound := map[string]bool{}

	// Mock current changes cache.
	cc := &caches_mocks.CurrentChangesCache{}
	cc.On("GetCurrentChangesCache", testutils.AnyContext, dbClient).Return(currentChangesCache).Once()
	cc.On("IsDryRun", testutils.AnyContext, dbClient).Return(false).Once()

	// Mock cfg reader.
	skcfg := &config.SkCQCfg{}
	cfgReader := &cfg_mocks.ConfigReader{}
	cfgReader.On("GetSkCQCfg", testutils.AnyContext).Return(skcfg).Once()

	// Mock codereview.
	cr := &cr_mocks.CodeReview{}
	// cr.On("GetIssueProperties", testutils.AnyContext, ci.Issue).Return(ci, nil).Once()
	cr.On("IsDryRun", testutils.AnyContext, ci).Return(true).Once()
	cr.On("IsCQ", testutils.AnyContext, ci).Return(false).Once()
	// cr.On("GetEarliestEquivalentPatchSetID", ci).Return(int64(5)).Once()
	// cr.On("GetFileNames", testutils.AnyContext, ci).Return([]string{"dir1/file1"}]).Once()

	processCL(context.Background(), ci, cfgReader, clsInThisRound, cr, cc, httpClient, dbClient, nil, publicFEInstanceURL, corpFEInstanceURL)
}
