package examiner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/monorail/v3"
	monorail_mocks "go.skia.org/infra/go/monorail/v3/mocks"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/types"
	npm_mocks "go.skia.org/infra/npm-audit-mirror/go/types/mocks"
)

func testOneExaminationCycle(t *testing.T, trustedScopes []string, packageCreatedLessThanThreshold, examinerIssueNotFiledYet, examinerIssueClosedLessThanThreshold bool) {
	projectName := "test_project"
	packageName := "@scope/test_package"
	projectPackageKey := fmt.Sprintf("%s_%s", projectName, strings.Replace(packageName, "/", "_", -1))
	ctx := context.Background()

	now := time.Now().UTC()
	closedTime := now.Add(-fileExaminerIssueAfterThreshold)
	if examinerIssueClosedLessThanThreshold {
		closedTime = now.Add(-5 * time.Minute)
	}
	testIssueName := "projects/skia/issues/11111"
	testMonorailIssue := &monorail.MonorailIssue{
		Name:        testIssueName,
		CreatedTime: now,
		ClosedTime:  closedTime,
	}
	retDbEntry := &types.NpmAuditData{
		Created:   now,
		IssueName: testIssueName,
	}

	updatedIssueName := "projects/skia/issues/22222"
	updated := now.Add(5 * time.Minute)
	updatedMonorailIssue := &monorail.MonorailIssue{
		Name:        updatedIssueName,
		CreatedTime: updated,
	}

	monorailConfig := &config.MonorailConfig{
		InstanceName:    "test_project_monorail",
		Owner:           "superman@krypton.com",
		Labels:          []string{},
		ComponentDefIDs: []string{},
	}

	// Mock monorail service.
	monorailServiceMock := monorail_mocks.NewIMonorailService(t)

	// Mock DB client.
	mockDBClient := npm_mocks.NewNpmDB(t)

	// Mock HTTP client.
	packageCreatedTime := now.Add(-packageCreatedTimeCutoff)
	if packageCreatedLessThanThreshold {
		packageCreatedTime = now.Add(-5 * time.Minute)
	}
	testPackageResp, err := json.Marshal(&types.NpmPackage{
		Time: map[string]string{
			"created": packageCreatedTime.Format(time.RFC3339),
		},
	})
	require.NoError(t, err)
	mockHttpClient := mockhttpclient.NewURLMock()
	mockHttpClient.Mock("https://registry.npmjs.org/"+packageName, mockhttpclient.MockGetDialogue(testPackageResp))

	// Mock ProjectMirror.
	mockProjectMirror := npm_mocks.NewProjectMirror(t)
	mockProjectMirror.On("GetProjectName").Return(projectName)
	mockProjectMirror.On("GetDownloadedPackageNames").Return([]string{packageName}, nil).Once()

	packageHasTrustedScope := false
	for _, t := range trustedScopes {
		if strings.HasPrefix(packageName, t) {
			packageHasTrustedScope = true
			break
		}
	}

	if packageCreatedLessThanThreshold && !packageHasTrustedScope {
		if examinerIssueNotFiledYet {
			retDbEntry = nil
		} else {
			monorailServiceMock.On("GetIssue", testIssueName).Return(testMonorailIssue, nil).Once()
		}
		mockDBClient.On("GetFromDB", ctx, projectPackageKey).Return(retDbEntry, nil).Once()
		if !examinerIssueClosedLessThanThreshold {
			monorailServiceMock.On("MakeIssue", monorailConfig.InstanceName, monorailConfig.Owner, mock.AnythingOfType("string"), mock.AnythingOfType("string"), defaultIssueStatus, defaultIssuePriority, defaultIssueType, []string{monorail.RestrictViewGoogleLabelName}, monorailConfig.ComponentDefIDs, []string{defaultCCUser}).Return(updatedMonorailIssue, nil).Once()
			mockDBClient.On("PutInDB", ctx, projectPackageKey, updatedIssueName, updated.UTC()).Return(nil).Once()
		}
	}

	a := &DownloadedPackagesExaminer{
		trustedScopes:   trustedScopes,
		httpClient:      mockHttpClient.Client(),
		dbClient:        mockDBClient,
		projectMirror:   mockProjectMirror,
		monorailConfig:  monorailConfig,
		monorailService: monorailServiceMock,
	}
	a.oneExaminationCycle(ctx, metrics2.NewLiveness("test_npm_audit"))
}

func TestStartExamination_NoRepublishedPackageFound_DoNotFileBugs(t *testing.T) {

	testOneExaminationCycle(t, []string{}, false, false, false)
}

func TestStartExamination_RepublishedPackageFound_NoExaminerIssueFiledYet_NoErrors(t *testing.T) {

	testOneExaminationCycle(t, []string{}, true, true, false)
}

func TestStartExamination_RepublishedPackageFound_ExaminerIssueClosedLessThanThreshold_NoErrors(t *testing.T) {

	testOneExaminationCycle(t, []string{}, true, false, true)
}

func TestStartExamination_RepublishedPackageFoundInAllowedScopes_NoErrors(t *testing.T) {

	testOneExaminationCycle(t, []string{"@scope/"}, true, false, true)
}

func TestStartExamination_FullEndToEndFlow_NoErrors(t *testing.T) {

	testOneExaminationCycle(t, []string{}, true, false, false)
}
