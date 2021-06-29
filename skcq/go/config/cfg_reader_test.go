package config

import (
	"testing"

	"go.skia.org/infra/go/gerrit"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
)

func TestGetSkCQCfg_ModifiedInChange(t *testing.T) {
	ci := &gerrit.ChangeInfo{
		Issue: int64(123),
		Owner: &gerrit.Person{Email: "batman@gotham.com"},
	}
	ref := "refs/changes/22/401222/140"
	changedFiles := []string{"xyz/abc", SkCQCfgPath}

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetChangeRef", ci).Return(ref).Once()

	// Mock gitilesRepo.
	gitilesRepo := &gitiles_mocks.GitilesRepo{}
	gitilesRepo.On("ReadFileAtRef", testutils.AnyContext, SkCQCfgPath, ref).Return("contents", nil).Once()

	configReader := &GitilesConfigReader{
		gitilesRepo:           gitilesRepo,
		cr:                    cr,
		ci:                    ci,
		changedFiles:          changedFiles,
		canModifyCfgsOnTheFly: cria,
	}
}
