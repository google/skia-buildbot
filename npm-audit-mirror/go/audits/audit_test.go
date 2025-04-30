package audits

import (
	// "context"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/executil"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/types"
	npm_mocks "go.skia.org/infra/npm-audit-mirror/go/types/mocks"
)

func testOneAuditCycle(t *testing.T, noHighSeverityIssuesFound, auditIssueNotFiledYet, auditIssueClosedLessThanThreshold bool, auditDevDependencies bool) {
	projectName := "test_project"
	repoURL := "https://skia.googlesource.com/buildbot.git"
	gitBranch := "main"
	packageFilesDir := ""
	workDir := os.TempDir()

	now := time.Now().UTC().Truncate(time.Second)
	closedTime := now.Add(-fileAuditIssueAfterThreshold)
	if auditIssueClosedLessThanThreshold {
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
	mockGitilesRepo := &gitiles_mocks.GitilesRepo{}
	defer mockGitilesRepo.AssertExpectations(t)
	mockDBClient := &npm_mocks.NpmDB{}
	defer mockDBClient.AssertExpectations(t)

	if !noHighSeverityIssuesFound {
		if auditIssueNotFiledYet {
			retDbEntry = nil
		} else {
			issueTrackerServiceMock.On("GetIssue", testIssueId).Return(testIssue, nil).Once()
		}
		mockDBClient.On("GetFromDB", testutils.AnyContext, projectName).Return(retDbEntry, nil).Once()
		if !auditIssueClosedLessThanThreshold {
			issueTrackerServiceMock.On("MakeIssue", issueTrackerConfig.Owner, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(updatedIssue, nil).Once()
			mockDBClient.On("PutInDB", testutils.AnyContext, projectName, updatedIssueId, updated.UTC()).Return(nil).Once()
			mockGitilesRepo.On("URL").Return("mock-URL").Once()
		}
	}

	mockGitilesRepo.On("DownloadFileAtRef", testutils.AnyContext, "package.json", gitBranch, path.Join(workDir, "package.json")).Return(nil).Once()
	mockGitilesRepo.On("DownloadFileAtRef", testutils.AnyContext, "package-lock.json", gitBranch, path.Join(workDir, "package-lock.json")).Return(nil).Once()

	// Mock executil calls.
	var ctx context.Context
	if auditDevDependencies {
		ctx = executil.FakeTestsContext("Test_FakeExe_NPM_Audit_ReturnsTwoHighDevIssues")
	} else if noHighSeverityIssuesFound {
		ctx = executil.FakeTestsContext("Test_FakeExe_NPM_Audit_ReturnsNoHighIssues")
	} else {
		ctx = executil.FakeTestsContext("Test_FakeExe_NPM_Audit_ReturnsTwoHighIssues")
	}

	a := &NpmProjectAudit{
		projectName:          projectName,
		repoURL:              repoURL,
		gitBranch:            gitBranch,
		packageFilesDir:      packageFilesDir,
		workDir:              workDir,
		gitilesRepo:          mockGitilesRepo,
		dbClient:             mockDBClient,
		issueTrackerConfig:   issueTrackerConfig,
		issueTrackerService:  issueTrackerServiceMock,
		auditDevDependencies: auditDevDependencies,
	}
	a.oneAuditCycle(ctx, metrics2.NewLiveness("test_npm_audit"))
}

func TestStartAudit_NoHighSeverityIssuesFound_DoNotFileBugs(t *testing.T) {

	testOneAuditCycle(t, true, false, false, false)
}

func TestStartAudit_HighSeverityIssues_NoAuditIssueFiledYet_NoErrors(t *testing.T) {

	testOneAuditCycle(t, false, true, false, false)
}

func TestStartAudit_HighSeverityIssues_AuditIssueClosedLessThanThreshold_NoErrors(t *testing.T) {

	testOneAuditCycle(t, false, false, true, false)
}

func TestStartAudit_FullEndToEndFlow_NoErrors(t *testing.T) {

	testOneAuditCycle(t, false, false, false, false)
}

func TestStartAudit_HighSeverityIssues_AuditDevDependencies_NoErrors(t *testing.T) {

	testOneAuditCycle(t, false, false, false, true)
}

// This is not a real test, but a fake implementation of the executable in question.
// By convention, we prefix these with FakeExe to make that clear.
func Test_FakeExe_NPM_Audit_ReturnsTwoHighIssues(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"npm", "audit", "--json", "--audit-level=high", "--omit=dev"}, args)

	auditResp, err := json.Marshal(&types.NpmAuditOutput{
		Metadata: types.NpmAuditMetadata{
			Vulnerabilities: map[string]int{
				"low":      10,
				"moderate": 20,
				"high":     1,
				"critical": 1,
			},
		},
	})
	require.NoError(t, err)

	fmt.Println(string(auditResp))
	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}

// This is not a real test, but a fake implementation of the executable in question.
// By convention, we prefix these with FakeExe to make that clear.
func Test_FakeExe_NPM_Audit_ReturnsTwoHighDevIssues(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"npm", "audit", "--json", "--audit-level=high"}, args)

	auditResp, err := json.Marshal(&types.NpmAuditOutput{
		Metadata: types.NpmAuditMetadata{
			Vulnerabilities: map[string]int{
				"low":      10,
				"moderate": 20,
				"high":     1,
				"critical": 1,
			},
		},
	})
	require.NoError(t, err)

	fmt.Println(string(auditResp))
	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}

func Test_FakeExe_NPM_Audit_ReturnsNoHighIssues(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	require.Equal(t, []string{"npm", "audit", "--json", "--audit-level=high", "--omit=dev"}, args)

	auditResp, err := json.Marshal(&types.NpmAuditOutput{
		Metadata: types.NpmAuditMetadata{
			Vulnerabilities: map[string]int{
				"low":      10,
				"moderate": 20,
			},
		},
	})
	require.NoError(t, err)

	fmt.Println(string(auditResp))
	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}
