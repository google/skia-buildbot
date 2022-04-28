package verifiers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
	"go.skia.org/infra/skcq/go/config"
	cfg_mocks "go.skia.org/infra/skcq/go/config/mocks"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/skcq/go/types/mocks"
)

var (
	// Time used for testing.
	vmCurrentTime = time.Unix(1598467386, 0).UTC()
)

func testGetVerifier(t *testing.T, isCQ, isDryRun bool, submittedTogetherChange *gerrit.ChangeInfo, expectedVerifiers []string) {
	allowListName := "test-cria-committers"
	cfg := &config.SkCQCfg{
		TreeStatusURL: "test-tree-status-url",
		TasksJSONPath: "infra/bots/tasks.json",
	}
	if isCQ {
		cfg.CommitterList = allowListName
	} else {
		cfg.DryRunAccessList = allowListName
	}
	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	commitMsg := "Test commit message"

	// Setup httpClient mock.
	mockClient := mockhttpclient.NewURLMock()
	mockClient.Mock(fmt.Sprintf(allowed.GROUP_URL_TEMPLATE, allowListName), mockhttpclient.MockGetDialogue([]byte("{}")))

	// Setup codereview and cfg_reader mocks.
	cr := &cr_mocks.CodeReview{}
	cfgReader := &cfg_mocks.ConfigReader{}
	if submittedTogetherChange == nil {
		cr.On("GetCommitMessage", testutils.AnyContext, ci.Issue).Return(commitMsg, nil).Once()
		cr.On("IsCQ", testutils.AnyContext, ci).Return(isCQ).Once()
		cr.On("IsDryRun", testutils.AnyContext, ci).Return(isDryRun).Once()
		cr.On("GetSubmittedTogether", testutils.AnyContext, ci).Return(nil, nil).Once()
		cr.On("Url", int64(0)).Return("skia-review.googlesource.com").Once()
		cfgReader.On("GetTasksCfg", testutils.AnyContext, cfg.TasksJSONPath).Return(nil, nil).Once()
	} else {
		// Return a mock CQ config for the together change.
		cqCfg := &config.SkCQCfg{
			CommitterList:    "committer-list",
			DryRunAccessList: "dry-run-access-list",
			VisibilityType:   "public",
		}
		cqCfgResp, err := json.Marshal(cqCfg)
		require.NoError(t, err)
		encodedResp := base64.StdEncoding.EncodeToString(cqCfgResp)
		md := mockhttpclient.MockGetDialogue([]byte(encodedResp))
		md.ResponseHeader(gitiles.ModeHeader, "0777")
		md.ResponseHeader(gitiles.TypeHeader, "blob")
		mockClient.Mock("test-repo-url/+show//infra/skcq.json?format=TEXT", md)

		cr.On("GetRepoUrl", submittedTogetherChange).Return("test-repo-url").Once()
		cr.On("GetFileNames", testutils.AnyContext, submittedTogetherChange).Return([]string{"test-file-name"}, nil).Once()
		cr.On("GetCommitMessage", testutils.AnyContext, ci.Issue).Return(commitMsg, nil).Once()
		cr.On("GetCommitMessage", testutils.AnyContext, submittedTogetherChange.Issue).Return(commitMsg, nil).Once()
		cr.On("IsCQ", testutils.AnyContext, ci).Return(isCQ).Twice()
		cr.On("IsDryRun", testutils.AnyContext, ci).Return(isDryRun).Twice()
		cr.On("GetSubmittedTogether", testutils.AnyContext, ci).Return([]*gerrit.ChangeInfo{submittedTogetherChange}, nil).Twice()
		cr.On("Url", int64(0)).Return("skia-review.googlesource.com").Twice()
		cfgReader.On("GetTasksCfg", testutils.AnyContext, cfg.TasksJSONPath).Return(nil, nil).Twice()
	}

	vm := &SkCQVerifiersManager{
		httpClient:     mockClient.Client(),
		criaClient:     mockClient.Client(),
		cr:             cr,
		allowlistCache: map[string]allowed.Allow{},
	}
	verifiers, actualSubmittedTogether, err := vm.GetVerifiers(context.Background(), cfg, ci, false, cfgReader)
	require.Nil(t, err)
	if submittedTogetherChange == nil || isDryRun {
		require.Len(t, actualSubmittedTogether, 0)
	} else {
		require.Len(t, actualSubmittedTogether, 1)
		require.Equal(t, strconv.FormatInt(submittedTogetherChange.Issue, 10), actualSubmittedTogether[0])
	}
	require.Len(t, verifiers, len(expectedVerifiers))
	for i, name := range expectedVerifiers {
		require.Equal(t, name, verifiers[i].Name())
	}
}

func TestGetVerifier_CQ_NoTogetherChanges(t *testing.T) {
	unittest.SmallTest(t)

	expectedVerifiers := []string{"CQAccessListVerifier", "CommitFooterVerifier", "WIPVerifier", "SubmittableVerifier", "TreeStatusVerifier", "ThrottlerVerifier", "TryJobsVerifier"}
	testGetVerifier(t, true, false, nil, expectedVerifiers)
}

func TestGetVerifier_CQ_WithTogetherChanges(t *testing.T) {
	unittest.SmallTest(t)

	expectedVerifiers := []string{"CQAccessListVerifier", "SubmittedTogetherVerifier", "CommitFooterVerifier", "WIPVerifier", "SubmittableVerifier", "TreeStatusVerifier", "ThrottlerVerifier", "TryJobsVerifier"}
	togetherChange := &gerrit.ChangeInfo{Issue: int64(222)}
	testGetVerifier(t, true, false, togetherChange, expectedVerifiers)
}

func TestGetVerifier_DryRun(t *testing.T) {
	unittest.SmallTest(t)

	expectedVerifiers := []string{"DryRunAccessListVerifier", "TryJobsVerifier"}
	testGetVerifier(t, false, true, nil, expectedVerifiers)
}

func TestGetVerifier_DryRun_WithTogetherChanges(t *testing.T) {
	unittest.SmallTest(t)

	// Together changes should be ignored for dry runs.
	expectedVerifiers := []string{"DryRunAccessListVerifier", "TryJobsVerifier"}
	togetherChange := &gerrit.ChangeInfo{Issue: int64(222)}
	testGetVerifier(t, false, true, togetherChange, expectedVerifiers)
}
func TestGetVerifier_CQAndDryRun(t *testing.T) {
	unittest.SmallTest(t)

	// If both CQ and DryRun are set then the verifiers should be the same as
	// CQ verifiers.
	expectedVerifiers := []string{"CQAccessListVerifier", "CommitFooterVerifier", "WIPVerifier", "SubmittableVerifier", "TreeStatusVerifier", "ThrottlerVerifier", "TryJobsVerifier"}
	testGetVerifier(t, true, true, nil, expectedVerifiers)
}

func TestRunVerifiers(t *testing.T) {
	unittest.SmallTest(t)

	ci := &gerrit.ChangeInfo{Issue: int64(123)}
	startTime := vmCurrentTime.Unix() - 100
	reason1 := "verify1 succeeded"
	reason2 := "verify2 is waiting"
	reason3 := "verify3 failed"
	vmTimeNowFunc = func() time.Time {
		return currentTime
	}

	// Setup mock for verifiers.
	v1 := &mocks.Verifier{}
	v1.On("Verify", testutils.AnyContext, ci, startTime).Return(types.VerifierSuccessState, reason1, nil).Once()
	v1.On("Name").Return("verifier1").Once()
	v2 := &mocks.Verifier{}
	v2.On("Verify", testutils.AnyContext, ci, startTime).Return(types.VerifierWaitingState, reason2, nil).Once()
	v2.On("Name").Return("verifier2").Once()
	v3 := &mocks.Verifier{}
	v3.On("Verify", testutils.AnyContext, ci, startTime).Return(types.VerifierFailureState, reason3, nil).Once()
	v3.On("Name").Return("verifier3").Once()
	// v3 will throw a transient error.
	v4 := &mocks.Verifier{}
	v4.On("Verify", testutils.AnyContext, ci, startTime).Return(types.VerifierState(""), "", skerr.Fmt("Error")).Once()
	v4.On("Name").Return("verifier4").Twice()

	vm := &SkCQVerifiersManager{}
	statuses := vm.RunVerifiers(context.Background(), ci, []types.Verifier{v1, v2, v3, v4}, startTime)
	require.Len(t, statuses, 4)
	// Assert the successful verifier.
	require.Equal(t, types.VerifierSuccessState, statuses[0].State)
	require.Equal(t, reason1, statuses[0].Reason)
	require.Equal(t, startTime, statuses[0].StartTs)
	require.Equal(t, currentTime.Unix(), statuses[0].StopTs)
	// Assert the waiting verifier.
	require.Equal(t, types.VerifierWaitingState, statuses[1].State)
	require.Equal(t, reason2, statuses[1].Reason)
	require.Equal(t, startTime, statuses[1].StartTs)
	require.Equal(t, int64(0), statuses[1].StopTs)
	// Assert the failed verifier.
	require.Equal(t, types.VerifierFailureState, statuses[2].State)
	require.Equal(t, reason3, statuses[2].Reason)
	require.Equal(t, startTime, statuses[2].StartTs)
	require.Equal(t, currentTime.Unix(), statuses[2].StopTs)
	// Assert the verifier that had an error
	require.Equal(t, types.VerifierWaitingState, statuses[3].State)
	require.Equal(t, startTime, statuses[3].StartTs)
	require.Equal(t, int64(0), statuses[3].StopTs)
}
