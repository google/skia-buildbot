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

	"go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/types"
	npm_mocks "go.skia.org/infra/npm-audit-mirror/go/types/mocks"
)

func testOneExaminationCycle(t *testing.T, trustedScopes []string, packageCreatedLessThanThreshold, examinerIssueNotFiledYet, examinerIssueClosedLessThanThreshold bool) {
	projectName := "test_project"
	packageName := "@scope/test_package"
	projectPackageKey := fmt.Sprintf("%s_%s", projectName, strings.Replace(packageName, "/", "_", -1))
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	closedTime := now.Add(-fileExaminerIssueAfterThreshold)
	if examinerIssueClosedLessThanThreshold {
		closedTime = now.Add(-5 * time.Minute)
	}
	testIssueId := int64(11111)
	testIssue := &issuetracker.Issue{
		IssueId:      testIssueId,
		CreatedTime:  now.Format(time.RFC3339),
		ResolvedTime: closedTime.Format(time.RFC3339),
	}
	retDbEntry := &types.NpmAuditData{
		Created: now,
		IssueId: testIssueId,
	}

	updatedIssueId := int64(22222)
	updated := now.Add(5 * time.Minute)
	updatedIssue := &issuetracker.Issue{
		IssueId:     updatedIssueId,
		CreatedTime: updated.Format(time.RFC3339),
	}

	issueTrackerConfig := &config.IssueTrackerConfig{
		Owner: "superman@krypton.com",
	}

	// Mock issue tracker service.
	issueTrackerServiceMock := npm_mocks.NewIIssueTrackerService(t)
	defer issueTrackerServiceMock.AssertExpectations(t)

	// Mock DB client.
	mockDBClient := npm_mocks.NewNpmDB(t)
	defer mockDBClient.AssertExpectations(t)

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
	defer mockProjectMirror.AssertExpectations(t)

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
			issueTrackerServiceMock.On("GetIssue", testIssueId).Return(testIssue, nil).Once()
		}
		mockDBClient.On("GetFromDB", ctx, projectPackageKey).Return(retDbEntry, nil).Once()
		if !examinerIssueClosedLessThanThreshold {
			issueTrackerServiceMock.On("MakeIssue", issueTrackerConfig.Owner, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(updatedIssue, nil).Once()
			mockDBClient.On("PutInDB", ctx, projectPackageKey, updatedIssueId, updated.UTC()).Return(nil).Once()
		}
	}

	a := &DownloadedPackagesExaminer{
		trustedScopes:       trustedScopes,
		httpClient:          mockHttpClient.Client(),
		dbClient:            mockDBClient,
		projectMirror:       mockProjectMirror,
		issueTrackerConfig:  issueTrackerConfig,
		issueTrackerService: issueTrackerServiceMock,
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
