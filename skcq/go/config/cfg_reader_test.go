package config

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
)

func TestGetSkCQCfg_ModifiedInChange(t *testing.T) {
	match := true
	matchedUser := "batman@gotham.com"
	unmatchedUser := "superman@krypton.com"

	changeOwner := unmatchedUser
	if match {
		changeOwner = matchedUser
	}

	ci := &gerrit.ChangeInfo{
		Issue: int64(123),
		Owner: &gerrit.Person{Email: changeOwner},
	}
	ref := "refs/changes/22/401222/140"
	changedFiles := []string{"xyz/abc", SkCQCfgPath}
	allowListName := "test-group-name"
	treeStatusURL := "http://tree-status-url"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetChangeRef", ci).Return(ref).Once()

	// Mock gitilesRepo.
	skCfg := &SkCQCfg{
		TreeStatusURL:    treeStatusURL,
		CommitterList:    "test-list",
		DryRunAccessList: "test-list",
	}
	skCfgContents, err := json.Marshal(skCfg)
	require.Nil(t, err)

	gitilesRepo := &gitiles_mocks.GitilesRepo{}
	// RETURN ERROR HERE ONCE.
	gitilesRepo.On("ReadFileAtRef", testutils.AnyContext, SkCQCfgPath, ref).Return(skCfgContents, nil).Once()

	// Mock httpClient for allowlist.
	mockClient := mockhttpclient.NewURLMock()
	mockClient.Mock(fmt.Sprintf(allowed.GROUP_URL_TEMPLATE, allowListName), mockhttpclient.MockGetDialogue([]byte(fmt.Sprintf(`{"group": {"members": ["user:%s"]}}`, matchedUser))))
	cria, err := allowed.NewAllowedFromChromeInfraAuth(mockClient.Client(), allowListName)
	require.Nil(t, err)

	configReader := &GitilesConfigReader{
		gitilesRepo:           gitilesRepo,
		cr:                    cr,
		ci:                    ci,
		changedFiles:          changedFiles,
		canModifyCfgsOnTheFly: cria,
	}
	cfg, err := configReader.GetSkCQCfg(context.Background())
	require.Nil(t, err)
	require.Equal(t, treeStatusURL, cfg.TreeStatusURL)

}
