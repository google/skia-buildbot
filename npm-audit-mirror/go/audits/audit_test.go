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
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/monorail/v3"
	monorail_mocks "go.skia.org/infra/go/monorail/v3/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/npm-audit-mirror/go/config"
	"go.skia.org/infra/npm-audit-mirror/go/types"
	npm_mocks "go.skia.org/infra/npm-audit-mirror/go/types/mocks"
)

func testOneAuditCycle(t *testing.T, noHighSeverityIssuesFound, auditIssueNotFiledYet, auditIssueClosedLessThanThreshold bool) {
	projectName := "test_project"
	repoURL := "https://skia.googlesource.com/buildbot.git"
	gitBranch := "main"
	packageFilesDir := ""
	workDir := os.TempDir()

	now := time.Now().UTC()
	closedTime := now.Add(-fileAuditIssueAfterThreshold)
	if auditIssueClosedLessThanThreshold {
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

	// Create Mocks.
	monorailServiceMock := &monorail_mocks.IMonorailService{}
	defer monorailServiceMock.AssertExpectations(t)
	mockGitilesRepo := &gitiles_mocks.GitilesRepo{}
	defer mockGitilesRepo.AssertExpectations(t)
	mockDBClient := &npm_mocks.NpmDB{}
	defer mockDBClient.AssertExpectations(t)

	if !noHighSeverityIssuesFound {
		if auditIssueNotFiledYet {
			retDbEntry = nil
		} else {
			monorailServiceMock.On("GetIssue", testIssueName).Return(testMonorailIssue, nil).Once()
		}
		mockDBClient.On("GetFromDB", testutils.AnyContext, projectName).Return(retDbEntry, nil).Once()
		if !auditIssueClosedLessThanThreshold {
			monorailServiceMock.On("MakeIssue", monorailConfig.InstanceName, monorailConfig.Owner, mock.AnythingOfType("string"), mock.AnythingOfType("string"), defaultIssueStatus, defaultIssuePriority, defaultIssueType, []string{monorail.RestrictViewGoogleLabelName}, monorailConfig.ComponentDefIDs, []string{defaultCCUser}).Return(updatedMonorailIssue, nil).Once()
			mockDBClient.On("PutInDB", testutils.AnyContext, projectName, updatedIssueName, updated.UTC()).Return(nil).Once()
			mockGitilesRepo.On("URL").Return("mock-URL").Once()
		}
	}

	mockGitilesRepo.On("DownloadFileAtRef", testutils.AnyContext, "package.json", gitBranch, path.Join(workDir, "package.json")).Return(nil).Once()
	mockGitilesRepo.On("DownloadFileAtRef", testutils.AnyContext, "package-lock.json", gitBranch, path.Join(workDir, "package-lock.json")).Return(nil).Once()

	// Mock executil calls.
	var ctx context.Context
	if noHighSeverityIssuesFound {
		ctx = executil.FakeTestsContext("Test_FakeExe_NPM_Audit_ReturnsNoHighIssues")
	} else {
		ctx = executil.FakeTestsContext("Test_FakeExe_NPM_Audit_ReturnsTwoHighIssues")
	}

	a := &NpmProjectAudit{
		projectName:     projectName,
		repoURL:         repoURL,
		gitBranch:       gitBranch,
		packageFilesDir: packageFilesDir,
		workDir:         workDir,
		gitilesRepo:     mockGitilesRepo,
		dbClient:        mockDBClient,
		monorailConfig:  monorailConfig,
		monorailService: monorailServiceMock,
	}
	a.oneAuditCycle(ctx, metrics2.NewLiveness("test_npm_audit"))
}

func TestStartAudit_NoHighSeverityIssuesFound_DoNotFileBugs(t *testing.T) {
	unittest.SmallTest(t)

	testOneAuditCycle(t, true, false, false)
}

func TestStartAudit_HighSeverityIssues_NoAuditIssueFiledYet_NoErrors(t *testing.T) {
	unittest.SmallTest(t)

	testOneAuditCycle(t, false, true, false)
}

func TestStartAudit_HighSeverityIssues_AuditIssueClosedLessThanThreshold_NoErrors(t *testing.T) {
	unittest.SmallTest(t)

	testOneAuditCycle(t, false, false, true)
}

func TestStartAudit_FullEndToEndFlow_NoErrors(t *testing.T) {
	unittest.SmallTest(t)

	testOneAuditCycle(t, false, false, false)
}

// This is not a real test, but a fake implementation of the executable in question.
// By convention, we prefix these with FakeExe to make that clear.
func Test_FakeExe_NPM_Audit_ReturnsTwoHighIssues(t *testing.T) {
	unittest.FakeExeTest(t)
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
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

	fmt.Printf(string(auditResp))
	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}

func Test_FakeExe_NPM_Audit_ReturnsNoHighIssues(t *testing.T) {
	unittest.FakeExeTest(t)
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if os.Getenv(executil.OverrideEnvironmentVariable) == "" {
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
			},
		},
	})
	require.NoError(t, err)

	fmt.Printf(string(auditResp))
	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}
