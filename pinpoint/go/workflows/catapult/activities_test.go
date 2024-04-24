package catapult

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/pinpoint/go/midpoint"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"

	_ "embed"
)

func TestFetchCommitActivity_ValidCommit_CommitWithAdditionalFields(t *testing.T) {
	commitHash := "493a946"
	commitRepository := midpoint.ChromiumSrcGit
	timeNow := time.Now().UTC()
	longCommitResp := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    commitHash,
			Author:  "John Doe (johndoe@gmail.com)",
			Subject: "[anchor-position] Implements resolving anchor-center.",
		},
		// var defined in parser_test.go
		Body:      mainCommitMsg,
		Timestamp: timeNow,
	}
	mockRepo := &mocks.GitilesRepo{}
	mockRepo.On("Details", testutils.AnyContext, commitHash).Return(longCommitResp, nil)

	ctx := context.Background()
	ctx = context.WithValue(ctx, httpClientKey, map[string]gitiles.GitilesRepo{commitRepository: mockRepo})

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment().SetWorkerOptions(worker.Options{BackgroundActivityContext: ctx})
	env.RegisterActivity(FetchCommitActivity)

	commit := &pinpoint_proto.Commit{
		Repository: commitRepository,
		GitHash:    commitHash,
	}
	res, err := env.ExecuteActivity(FetchCommitActivity, commit)
	require.NoError(t, err)

	var actual *vcsinfo.LongCommit
	err = res.Get(&actual)
	require.NoError(t, err)

	assert.Equal(t, mainCommitMsg, actual.Body)
	assert.Equal(t, commitHash, actual.ShortCommit.Hash)
}
