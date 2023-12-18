package midpoint

import (
	"context"
	"strconv"
	"testing"

	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"

	. "github.com/smartystreets/goconvey/convey"
	. "go.chromium.org/luci/common/testing/assertions"
)

// generateCommitResponse will create a LongCommit slice response for gitiles.Repo.LogLinear.
func generateCommitResponse(num int) []*vcsinfo.LongCommit {
	resp := make([]*vcsinfo.LongCommit, 0)

	for i := num; i >= 1; i-- {
		resp = append(resp, &vcsinfo.LongCommit{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: strconv.Itoa(i),
			},
		})
	}

	return resp
}

func TestGetMidpoint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	repoUrl := "https://chromium.org/chromium/src"

	startGitHash := "1"
	endGitHash := "5"

	Convey(`Invalid`, t, func() {
		Convey(`Error thrown`, func() {
			gc := &mocks.GitilesRepo{}
			gc.On("LogLinear", testutils.AnyContext, startGitHash, endGitHash).Return(nil, skerr.Fmt("fail!"))
			commit, err := GetMidpoint(ctx, gc, repoUrl, startGitHash, endGitHash)

			So(err, ShouldNotBeNil)
			So(commit, ShouldBeNil)
		})

		Convey(`Empty repsonse`, func() {
			gc := &mocks.GitilesRepo{}
			resp := generateCommitResponse(0)
			gc.On("LogLinear", testutils.AnyContext, startGitHash, endGitHash).Return(resp, nil)

			commit, err := GetMidpoint(ctx, gc, repoUrl, startGitHash, endGitHash)
			So(commit, ShouldBeNil)
			So(err, ShouldErrLike, GITILES_EMPTY_RESP_ERROR)
		})

		// TODO(jeffyoon@): Requires updates once DEPS parsing is implemented.
		Convey(`Single response`, func() {
			gc := &mocks.GitilesRepo{}
			resp := make([]*vcsinfo.LongCommit, 0)
			resp = append(resp, &vcsinfo.LongCommit{
				ShortCommit: &vcsinfo.ShortCommit{
					Hash: "2",
				},
			})
			gc.On("LogLinear", testutils.AnyContext, startGitHash, "2").Return(resp, nil)
			commit, err := GetMidpoint(ctx, gc, repoUrl, startGitHash, "2")
			// The base git hash to apply DEPS changes is on start git hash.
			So(commit.GitHash, ShouldEqual, "1")
			So(commit.RepositoryUrl, ShouldEqual, repoUrl)
			So(err, ShouldBeNil)
		})
	})

	Convey(`E2E`, t, func() {
		Convey(`Even response`, func() {
			gc := &mocks.GitilesRepo{}
			validResp := generateCommitResponse(5)

			gc.On("LogLinear", testutils.AnyContext, startGitHash, endGitHash).Return(validResp, nil)

			commit, err := GetMidpoint(ctx, gc, repoUrl, startGitHash, endGitHash)
			So(err, ShouldBeNil)
			So(commit.RepositoryUrl, ShouldEqual, repoUrl)

			// endGitHash is popped off, leaving [1, 2, 3, 4]
			// and since len == 4, mid index == 2
			So(commit.GitHash, ShouldEqual, "3")
		})

		Convey(`Odd response`, func() {
			gc := &mocks.GitilesRepo{}
			validResp := generateCommitResponse(6)
			gc.On("LogLinear", testutils.AnyContext, startGitHash, endGitHash).Return(validResp, nil)

			commit, err := GetMidpoint(ctx, gc, repoUrl, startGitHash, endGitHash)
			So(err, ShouldBeNil)
			So(commit.RepositoryUrl, ShouldEqual, repoUrl)

			// endGitHash is popped off, leaving [1, 2, 3, 4, 5]
			// and since len == 5, mid index == 2
			So(commit.GitHash, ShouldEqual, "3")
		})
	})
}
