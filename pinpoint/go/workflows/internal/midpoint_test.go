package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"

	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/midpoint"

	_ "embed"
)

//go:embed testdata/LatestChromiumDEPS
var LatestChromiumDEPS string

//go:embed testdata/LatestV8DEPS
var LatestV8DEPS string

//go:embed testdata/NMinusOneChromiumDEPS
var NMinusOneChromiumDEPS string

//go:embed testdata/NMinusOneV8DEPS
var NMinusOneV8DEPS string

const (
	LatestChromiumGitHash    = "f8e1800"
	NMinusOneChromiumGitHash = "836476df"
	NMinusTwoChromiumGitHash = "93dd4db"

	V8Url              = "https://chromium.googlesource.com/v8/v8"
	LatestV8GitHash    = "21b24dd"
	NMinusOneV8GitHash = "ae02432"
	NMinusTwoV8GitHash = "385416a"

	SkiaUrl              = "https://skia.googlesource.com/skia"
	LatestSkiaGitHash    = "14dd552"
	NMinusOneSkiaGitHash = "0335263"
	NMinusTwoSkiaGitHash = "3e3f28d"
)

func createShortCommit(gitHash string) *vcsinfo.ShortCommit {
	return &vcsinfo.ShortCommit{
		Hash: gitHash,
	}
}

func createResponse(commit ...*vcsinfo.ShortCommit) []*vcsinfo.LongCommit {
	resp := make([]*vcsinfo.LongCommit, 0)
	for _, c := range commit {
		resp = append(resp, &vcsinfo.LongCommit{
			ShortCommit: c,
		})
	}
	return resp
}

func runFindMidCommitActivity(t *testing.T, ctx context.Context, lower, higher *common.CombinedCommit) *common.CombinedCommit {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment().SetWorkerOptions(worker.Options{BackgroundActivityContext: ctx})
	env.RegisterActivity(FindMidCommitActivity)

	res, err := env.ExecuteActivity(FindMidCommitActivity, lower, higher)
	require.NoError(t, err)

	var actual *common.CombinedCommit
	err = res.Get(&actual)
	require.NoError(t, err)
	return actual
}

func runCombinedCommitEqualActivity(t *testing.T, ctx context.Context, first, second *common.CombinedCommit) bool {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment().SetWorkerOptions(worker.Options{BackgroundActivityContext: ctx})
	env.RegisterActivity(CheckCombinedCommitEqualActivity)

	res, err := env.ExecuteActivity(CheckCombinedCommitEqualActivity, first, second)
	require.NoError(t, err)

	var actual bool
	err = res.Get(&actual)
	require.NoError(t, err)
	return actual
}

// TODO(jeffyoon@) - all tests below can avoid context setting with internal handler behavior mocked
// if the handler itself is mocked.
func TestFindMidCommitActivity_NoModifiedDeps_MidpointInMain(t *testing.T) {
	ctx := context.Background()

	// n+2 vs latest, so midpoint should be n+1
	lower := common.NewCombinedCommit(common.NewChromiumCommit(NMinusTwoChromiumGitHash))
	higher := common.NewCombinedCommit(common.NewChromiumCommit(LatestChromiumGitHash))

	chromiumRepo := &mocks.GitilesRepo{}

	// The response always returns inclusive of the latest.
	logFirstParentResp := createResponse(
		createShortCommit(LatestChromiumGitHash),
		createShortCommit(NMinusOneChromiumGitHash),
	)
	chromiumRepo.On("LogFirstParent", testutils.AnyContext, NMinusTwoChromiumGitHash, LatestChromiumGitHash).Return(logFirstParentResp, nil)

	c := mockhttpclient.NewURLMock().Client()
	handler := midpoint.New(ctx, c).WithRepo(midpoint.ChromiumSrcGit, chromiumRepo)

	ctx = context.WithValue(ctx, MidpointHandlerContextKey, handler)

	actual := runFindMidCommitActivity(t, ctx, lower, higher)
	assert.Equal(t, NMinusOneChromiumGitHash, actual.Main.GitHash)
}

func TestFindMidCommitActivity_AdjacentMain_MidpointInDep(t *testing.T) {
	ctx := context.Background()

	// adjacent, so logic assumes a deps roll and parses deps for midpoint
	lower := common.NewCombinedCommit(common.NewChromiumCommit(NMinusOneChromiumGitHash))
	higher := common.NewCombinedCommit(common.NewChromiumCommit(LatestChromiumGitHash))

	chromiumRepo := &mocks.GitilesRepo{}

	adjacentChromiumResp := createResponse(
		createShortCommit(LatestChromiumGitHash),
	)
	chromiumRepo.On("LogFirstParent", testutils.AnyContext, NMinusOneChromiumGitHash, LatestChromiumGitHash).Return(adjacentChromiumResp, nil)

	// DEPS roll will be from N-2 to Latest
	chromiumRepo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", NMinusOneChromiumGitHash).Return([]byte(NMinusOneChromiumDEPS), nil)
	chromiumRepo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", LatestChromiumGitHash).Return([]byte(LatestChromiumDEPS), nil)

	v8Repo := &mocks.GitilesRepo{}

	// Midpoint between N-2 to Latest would return V8 midpoint of N-1
	adjacentV8Resp := createResponse(
		createShortCommit(LatestV8GitHash),
		createShortCommit(NMinusOneV8GitHash),
	)
	v8Repo.On("LogFirstParent", testutils.AnyContext, NMinusTwoV8GitHash, LatestV8GitHash).Return(adjacentV8Resp, nil)

	c := mockhttpclient.NewURLMock().Client()
	handler := midpoint.New(ctx, c).WithRepo(midpoint.ChromiumSrcGit, chromiumRepo).WithRepo(V8Url, v8Repo)

	ctx = context.WithValue(ctx, MidpointHandlerContextKey, handler)

	actual := runFindMidCommitActivity(t, ctx, lower, higher)

	// Main Git Hash should be off of lower, and the first ModifiedDeps entry
	// should be V8 at N-1
	assert.Equal(t, NMinusOneChromiumGitHash, actual.Main.GitHash)
	assert.Equal(t, 1, len(actual.ModifiedDeps))
	modifiedDep := actual.GetLatestModifiedDep()
	assert.Equal(t, NMinusOneV8GitHash, modifiedDep.GitHash)
}

func TestFindMidCommitActivity_AdjacentMain_NoMoreMidpoint(t *testing.T) {
	ctx := context.Background()

	// adjacent, so logic assumes a deps roll and parses deps for midpoint
	lower := common.NewCombinedCommit(common.NewChromiumCommit(NMinusOneChromiumGitHash))
	higher := common.NewCombinedCommit(common.NewChromiumCommit(LatestChromiumGitHash))

	chromiumRepo := &mocks.GitilesRepo{}

	adjacentChromiumResp := createResponse(
		createShortCommit(LatestChromiumGitHash),
	)
	chromiumRepo.On("LogFirstParent", testutils.AnyContext, NMinusOneChromiumGitHash, LatestChromiumGitHash).Return(adjacentChromiumResp, nil)

	// return the same deps content at both commits
	chromiumRepo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", NMinusOneChromiumGitHash).Return([]byte(LatestChromiumDEPS), nil)
	chromiumRepo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", LatestChromiumGitHash).Return([]byte(LatestChromiumDEPS), nil)

	c := mockhttpclient.NewURLMock().Client()
	handler := midpoint.New(ctx, c).WithRepo(midpoint.ChromiumSrcGit, chromiumRepo)

	ctx = context.WithValue(ctx, MidpointHandlerContextKey, handler)

	actual := runFindMidCommitActivity(t, ctx, lower, higher)

	// When there's no midpoint, the lower commit is returned.
	assert.Equal(t, lower.Main.GitHash, actual.Main.GitHash)
	assert.Nil(t, actual.ModifiedDeps)
}

func TestFindMidCommitActivity_LowerNoModifiedDeps_MidpointInDep(t *testing.T) {
	ctx := context.Background()

	// same base commit, but different modified deps length, so that lower gets
	// backfilled.
	lower := common.NewCombinedCommit(common.NewChromiumCommit(NMinusOneChromiumGitHash))
	higher := common.NewCombinedCommit(
		common.NewChromiumCommit(NMinusOneChromiumGitHash),
		common.NewCommit(V8Url, LatestV8GitHash),
	)

	// Since lower has no modified deps, it's backfilled starting at the Main commit's git hash.
	// When fetching DEPS content, return the N-2 V8 hash so that midpoint returned is N-1.
	chromiumRepo := &mocks.GitilesRepo{}
	chromiumRepo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", NMinusOneChromiumGitHash).Return([]byte(NMinusOneChromiumDEPS), nil)

	v8Repo := &mocks.GitilesRepo{}
	v8LogFirstParentResp := createResponse(
		createShortCommit(LatestV8GitHash),
		createShortCommit(NMinusOneV8GitHash),
	)
	v8Repo.On("LogFirstParent", testutils.AnyContext, NMinusTwoV8GitHash, LatestV8GitHash).Return(v8LogFirstParentResp, nil)

	c := mockhttpclient.NewURLMock().Client()
	handler := midpoint.New(ctx, c).WithRepo(midpoint.ChromiumSrcGit, chromiumRepo).WithRepo(V8Url, v8Repo)

	ctx = context.WithValue(ctx, MidpointHandlerContextKey, handler)

	actual := runFindMidCommitActivity(t, ctx, lower, higher)

	// Main git hash should remain the same
	assert.Equal(t, lower.Main.GitHash, actual.Main.GitHash)
	assert.Equal(t, 1, len(actual.ModifiedDeps))

	actualModifiedDep := actual.GetLatestModifiedDep()
	assert.Equal(t, NMinusOneV8GitHash, actualModifiedDep.GitHash)
}

func TestFindMidCommitActivity_AdjancentModifiedDeps_MidpointInDep(t *testing.T) {
	ctx := context.Background()

	// adjacent modified deps
	lower := common.NewCombinedCommit(
		common.NewChromiumCommit(NMinusOneChromiumGitHash),
		common.NewCommit(V8Url, NMinusOneV8GitHash),
	)
	higher := common.NewCombinedCommit(
		common.NewChromiumCommit(NMinusOneChromiumGitHash),
		common.NewCommit(V8Url, LatestV8GitHash),
	)

	// Return adjacent. It'll need to traverse DEPS.
	v8Repo := &mocks.GitilesRepo{}
	v8LogFirstParentResp := createResponse(
		createShortCommit(LatestV8GitHash),
	)
	v8Repo.On("LogFirstParent", testutils.AnyContext, NMinusOneV8GitHash, LatestV8GitHash).Return(v8LogFirstParentResp, nil)
	v8Repo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", NMinusOneV8GitHash).Return([]byte(NMinusOneV8DEPS), nil)
	v8Repo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", LatestV8GitHash).Return([]byte(LatestV8DEPS), nil)

	skiaRepo := &mocks.GitilesRepo{}
	skiaLogFirstParentResp := createResponse(
		createShortCommit(LatestSkiaGitHash),
		createShortCommit(NMinusOneSkiaGitHash),
	)
	// DEPS from V8 would give N-2 to Latest for Skia Git Hash, and the midpoint should be N-1.
	skiaRepo.On("LogFirstParent", testutils.AnyContext, NMinusTwoSkiaGitHash, LatestSkiaGitHash).Return(skiaLogFirstParentResp, nil)

	c := mockhttpclient.NewURLMock().Client()
	handler := midpoint.New(ctx, c).WithRepo(V8Url, v8Repo).WithRepo(SkiaUrl, skiaRepo)

	ctx = context.WithValue(ctx, MidpointHandlerContextKey, handler)

	actual := runFindMidCommitActivity(t, ctx, lower, higher)

	assert.Equal(t, 2, len(actual.ModifiedDeps))
	// first modified dep should be lower of v8
	actualFirstModifiedDep := actual.ModifiedDeps[0]
	assert.Equal(t, V8Url, actualFirstModifiedDep.Repository)
	assert.Equal(t, NMinusOneV8GitHash, actualFirstModifiedDep.GitHash)
	// next modified dep should be skia
	latestModifiedDep := actual.GetLatestModifiedDep()
	assert.Equal(t, SkiaUrl, latestModifiedDep.Repository)
	assert.Equal(t, NMinusOneSkiaGitHash, latestModifiedDep.GitHash)
}

func TestFindMidCommitActivity_AdjancentModifiedDeps_NoMoreMidpoint(t *testing.T) {
	ctx := context.Background()

	// adjacent modified deps
	lower := common.NewCombinedCommit(
		common.NewChromiumCommit(NMinusOneChromiumGitHash),
		common.NewCommit(V8Url, NMinusOneV8GitHash),
	)
	higher := common.NewCombinedCommit(
		common.NewChromiumCommit(NMinusOneChromiumGitHash),
		common.NewCommit(V8Url, LatestV8GitHash),
	)

	// Return adjacent. It'll need to traverse DEPS.
	v8Repo := &mocks.GitilesRepo{}
	v8LogFirstParentResp := createResponse(
		createShortCommit(LatestV8GitHash),
	)
	v8Repo.On("LogFirstParent", testutils.AnyContext, NMinusOneV8GitHash, LatestV8GitHash).Return(v8LogFirstParentResp, nil)
	v8Repo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", NMinusOneV8GitHash).Return([]byte(NMinusOneV8DEPS), nil)
	// return the same DEPS file so that there's nothing more to traverse
	v8Repo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", LatestV8GitHash).Return([]byte(NMinusOneV8DEPS), nil)

	c := mockhttpclient.NewURLMock().Client()
	handler := midpoint.New(ctx, c).WithRepo(V8Url, v8Repo)

	ctx = context.WithValue(ctx, MidpointHandlerContextKey, handler)

	actual := runFindMidCommitActivity(t, ctx, lower, higher)
	// response should be equal to lower
	assert.Equal(t, 1, len(actual.ModifiedDeps))
	latestModifiedDep := actual.GetLatestModifiedDep()
	assert.Equal(t, V8Url, latestModifiedDep.Repository)
	assert.Equal(t, NMinusOneV8GitHash, latestModifiedDep.GitHash)
}

func TestCheckCombinedCommitEqual_NoModifiedDeps_Equal(t *testing.T) {
	ctx := context.Background()

	first := common.NewCombinedCommit(common.NewChromiumCommit(LatestChromiumGitHash))
	second := common.NewCombinedCommit(common.NewChromiumCommit(LatestChromiumGitHash))

	isEqual := runCombinedCommitEqualActivity(t, ctx, first, second)
	assert.True(t, isEqual)
}

func TestCheckCombinedCommitEqual_NoModifiedDeps_NotEqual(t *testing.T) {
	ctx := context.Background()

	first := common.NewCombinedCommit(common.NewChromiumCommit(NMinusTwoChromiumGitHash))
	second := common.NewCombinedCommit(common.NewChromiumCommit(LatestChromiumGitHash))

	isEqual := runCombinedCommitEqualActivity(t, ctx, first, second)
	assert.False(t, isEqual)
}

func TestCheckCombinedCommitEqual_UnevenModifiedDeps_Equal(t *testing.T) {
	ctx := context.Background()

	first := common.NewCombinedCommit(common.NewChromiumCommit(NMinusOneChromiumGitHash))
	second := common.NewCombinedCommit(
		common.NewChromiumCommit(NMinusOneChromiumGitHash),
		common.NewCommit(V8Url, NMinusTwoV8GitHash),
	)

	chromiumRepo := &mocks.GitilesRepo{}
	// DEPS at NMinusOneChromiumGitHash is NMinusTwoV8GitHash for V8
	// so should be equal.
	chromiumRepo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", NMinusOneChromiumGitHash).Return([]byte(NMinusOneChromiumDEPS), nil)

	c := mockhttpclient.NewURLMock().Client()
	handler := midpoint.New(ctx, c).WithRepo(midpoint.ChromiumSrcGit, chromiumRepo)
	ctx = context.WithValue(ctx, MidpointHandlerContextKey, handler)

	isEqual := runCombinedCommitEqualActivity(t, ctx, first, second)
	assert.True(t, isEqual)
}

func TestCheckCombinedCommitEqual_UnevenModifiedDeps_NotEqual(t *testing.T) {
	ctx := context.Background()

	first := common.NewCombinedCommit(common.NewChromiumCommit(NMinusOneChromiumGitHash))
	second := common.NewCombinedCommit(
		common.NewChromiumCommit(NMinusOneChromiumGitHash),
		common.NewCommit(V8Url, LatestV8GitHash),
	)

	chromiumRepo := &mocks.GitilesRepo{}
	// DEPS at NMinusOneChromiumGitHash is NMinusTwoV8GitHash for V8
	// so should be different since second has V8 set to LatestV8GitHash.
	chromiumRepo.On("ReadFileAtRef", testutils.AnyContext, "DEPS", NMinusOneChromiumGitHash).Return([]byte(NMinusOneChromiumDEPS), nil)

	c := mockhttpclient.NewURLMock().Client()
	handler := midpoint.New(ctx, c).WithRepo(midpoint.ChromiumSrcGit, chromiumRepo)
	ctx = context.WithValue(ctx, MidpointHandlerContextKey, handler)

	isEqual := runCombinedCommitEqualActivity(t, ctx, first, second)
	assert.False(t, isEqual)
}
