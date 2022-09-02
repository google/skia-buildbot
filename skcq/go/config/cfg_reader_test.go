package config

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.skia.org/infra/task_scheduler/go/specs"

	"github.com/stretchr/testify/require"

	allowed_mocks "go.skia.org/infra/go/allowed/mocks"
	"go.skia.org/infra/go/gerrit"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
)

func setupGetSkCQCfg(t *testing.T, match bool, cfgContents []byte, readFileError error, changedFiles []string, cfgPath string, updatedInChange bool) *GitilesConfigReader {
	matchedUser := "batman@gotham.com"
	unmatchedUser := "superman@krypton.com"

	changeOwner := unmatchedUser
	if match {
		changeOwner = matchedUser
	}

	ci := &gerrit.ChangeInfo{
		Issue:   int64(123),
		Owner:   &gerrit.Person{Email: changeOwner},
		Project: "test-repo",
		Branch:  "test-branch",
	}
	ref := "refs/changes/22/401222/140"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	if updatedInChange {
		cr.On("GetChangeRef", ci).Return(ref).Once()
	}

	// Mock gitilesRepo.
	gitilesRepo := &gitiles_mocks.GitilesRepo{}
	if updatedInChange {
		gitilesRepo.On("ReadFileAtRef", testutils.AnyContext, cfgPath, ref).Return(cfgContents, readFileError).Once()
	} else {
		gitilesRepo.On("ReadFileAtRef", testutils.AnyContext, cfgPath, ci.Branch).Return(cfgContents, readFileError).Once()
	}

	// Mock allow.
	allow := &allowed_mocks.Allow{}
	allow.On("Member", matchedUser).Return(true).Once()
	allow.On("Member", unmatchedUser).Return(false).Once()

	return &GitilesConfigReader{
		gitilesRepo:           gitilesRepo,
		cr:                    cr,
		ci:                    ci,
		changedFiles:          changedFiles,
		canModifyCfgsOnTheFly: allow,
	}
}

func TestGetSkCQCfg_UpdatedInChange(t *testing.T) {

	treeStatusURL := "http://tree-status-url"
	skCfg := &SkCQCfg{
		VisibilityType:   InternalVisibility,
		TreeStatusURL:    treeStatusURL,
		CommitterList:    "test-list",
		DryRunAccessList: "test-list",
	}
	skCfgContents, err := json.Marshal(skCfg)
	require.Nil(t, err)

	configReader := setupGetSkCQCfg(t, true, skCfgContents, nil, []string{"dir1/*", SkCQCfgPath}, SkCQCfgPath, true)
	cfg, err := configReader.GetSkCQCfg(context.Background())
	require.Nil(t, err)
	require.Equal(t, treeStatusURL, cfg.TreeStatusURL)
	require.Equal(t, InternalVisibility, cfg.VisibilityType)
}

func TestGetSkCQCfg_UpdatedInChange_NotAllowed(t *testing.T) {

	configReader := setupGetSkCQCfg(t, false, []byte{}, nil, []string{"dir1/*", SkCQCfgPath}, SkCQCfgPath, true)
	cfg, err := configReader.GetSkCQCfg(context.Background())
	require.Nil(t, cfg)
	require.NotNil(t, err)
	_, ok := err.(*CannotModifyCfgsOnTheFlyError)
	require.True(t, ok)
}

func TestGetSkCQCfg_NotUpdatedInChange(t *testing.T) {

	treeStatusURL := "http://tree-status-url"
	skCfg := &SkCQCfg{
		VisibilityType:   PublicVisibility,
		TreeStatusURL:    treeStatusURL,
		CommitterList:    "test-list",
		DryRunAccessList: "test-list",
	}
	skCfgContents, err := json.Marshal(skCfg)
	require.Nil(t, err)

	configReader := setupGetSkCQCfg(t, true, skCfgContents, nil, []string{"dir1/*", "dir2/dir3/DEPS"}, SkCQCfgPath, false)
	cfg, err := configReader.GetSkCQCfg(context.Background())
	require.Nil(t, err)
	require.Equal(t, treeStatusURL, cfg.TreeStatusURL)
	require.Equal(t, PublicVisibility, cfg.VisibilityType)
}

func TestGetSkCQCfg_NotFound(t *testing.T) {

	configReader := setupGetSkCQCfg(t, true, []byte{}, errors.New("NOT_FOUND"), []string{"dir1/*", "dir2/dir3/DEPS"}, SkCQCfgPath, false)
	cfg, err := configReader.GetSkCQCfg(context.Background())
	require.Nil(t, cfg)
	require.NotNil(t, err)
	_, ok := err.(*ConfigNotFoundError)
	require.True(t, ok)
}

func TestGetSkCQCfg_ValidationFailed(t *testing.T) {

	treeStatusURL := "http://tree-status-url"
	// Missing CommitterList and DryRunAccessList.
	skCfg := &SkCQCfg{
		TreeStatusURL: treeStatusURL,
	}
	skCfgContents, err := json.Marshal(skCfg)
	require.Nil(t, err)

	configReader := setupGetSkCQCfg(t, true, skCfgContents, nil, []string{"dir1/*", "dir2/dir3/DEPS"}, SkCQCfgPath, false)
	cfg, err := configReader.GetSkCQCfg(context.Background())
	require.Nil(t, cfg)
	require.NotNil(t, err)
}

func TestGetTasksCfg_UpdatedInChange(t *testing.T) {

	tasksJSONPath := "test/path/tasks.json"
	tasksJSON := &specs.TasksCfg{}
	tasksJSONContents, err := json.Marshal(tasksJSON)
	require.Nil(t, err)

	configReader := setupGetSkCQCfg(t, true, tasksJSONContents, nil, []string{"dir1/*", tasksJSONPath}, tasksJSONPath, true)
	cfg, err := configReader.GetTasksCfg(context.Background(), tasksJSONPath)
	require.Nil(t, err)
	require.NotNil(t, cfg)
}

func TestGetTasksCfg_NotUpdatedInChange(t *testing.T) {

	tasksJSONPath := "test/path/tasks.json"
	tasksJSON := &specs.TasksCfg{}
	tasksJSONContents, err := json.Marshal(tasksJSON)
	require.Nil(t, err)

	configReader := setupGetSkCQCfg(t, true, tasksJSONContents, nil, []string{"dir1/*"}, tasksJSONPath, false)
	cfg, err := configReader.GetTasksCfg(context.Background(), tasksJSONPath)
	require.Nil(t, err)
	require.NotNil(t, cfg)
}
