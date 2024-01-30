package midpoint

import (
	"context"
	"strconv"
	"testing"

	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"

	. "github.com/smartystreets/goconvey/convey"
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

func TestFetchGitDeps(t *testing.T) {
	ctx := context.Background()

	chromium := "https://chromium.org/chromium/src"
	gitHash := "1"

	sampleDeps := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': 'deadbeef',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '1',
  'src/third_party/lighttpd': {
    'url': Var('chromium_git') + '/deps/lighttpd.git' + '@' + '9dfa55d',
    'condition': 'checkout_mac or checkout_win',
  },
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
  'src/third_party/intellij': {
    'packages': [{
      'package': 'chromium/third_party/intellij',
      'version': 'version:12.0-cr0',
    }],
    'condition': 'checkout_android',
    'dep_type': 'cipd',
  },
}
    `

	Convey(`Ensure CIPD based DEP not present`, t, func() {
		gc := &mocks.GitilesRepo{}
		gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", gitHash).Return([]byte(sampleDeps), nil)

		c := mockhttpclient.NewURLMock().Client()
		r := New(ctx, c).WithRepo(chromium, gc)

		res, err := r.fetchGitDeps(ctx, gc, gitHash)
		So(err, ShouldBeNil)
		// intellij should be missing
		So(len(res), ShouldEqual, 3)
		So(res["https://chromium.googlesource.com/v8/v8"], ShouldEqual, "1")
		So(res["https://chromium.googlesource.com/deps/lighttpd"], ShouldEqual, "9dfa55d")
		So(res["https://webrtc.googlesource.com/src"], ShouldEqual, "deadbeef")
	})
}

func TestDetermineNextCandidate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	chromium := "https://chromium.org/chromium/src"
	webrtc := "https://webrtc.googlesource.com/src"

	Convey(`OK`, t, func() {
		Convey(`Midpoint in Chromium from even number of commits`, func() {
			startGitHash := "1"
			endGitHash := "5"

			gc := &mocks.GitilesRepo{}
			validResp := generateCommitResponse(5)

			gc.On("LogLinear", testutils.AnyContext, startGitHash, endGitHash).Return(validResp, nil)

			c := mockhttpclient.NewURLMock().Client()
			r := New(ctx, c).WithRepo(chromium, gc)

			next, ranges, err := r.DetermineNextCandidate(ctx, chromium, startGitHash, endGitHash)
			So(err, ShouldBeNil)
			So(ranges, ShouldBeNil)

			So(next.Main.RepositoryUrl, ShouldEqual, chromium)

			// endGitHash is popped off, leaving [1, 2, 3, 4]
			// and since len == 4, mid index == 2
			So(next.Main.GitHash, ShouldEqual, "3")
		})

		sampleDeps := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': '1',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '1',
  'src/third_party/lighttpd': {
    'url': Var('chromium_git') + '/deps/lighttpd.git' + '@' + '9dfa55d',
    'condition': 'checkout_mac or checkout_win',
  },
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
  'src/third_party/intellij': {
    'packages': [{
      'package': 'chromium/third_party/intellij',
      'version': 'version:12.0-cr0',
    }],
    'condition': 'checkout_android',
    'dep_type': 'cipd',
  },
}
		`

		Convey(`Adjacent changes, but not a DEPS roll`, func() {
			startGitHash := "1"
			endGitHash := "2"
			resp := []*vcsinfo.LongCommit{
				{
					ShortCommit: &vcsinfo.ShortCommit{
						Hash: "2",
					},
				},
			}

			gc := &mocks.GitilesRepo{}
			gc.On("LogLinear", testutils.AnyContext, startGitHash, endGitHash).Return(resp, nil)

			gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", startGitHash).Return([]byte(sampleDeps), nil)
			gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", endGitHash).Return([]byte(sampleDeps), nil)

			c := mockhttpclient.NewURLMock().Client()
			r := New(ctx, c).WithRepo(chromium, gc)
			next, ranges, err := r.DetermineNextCandidate(ctx, chromium, startGitHash, endGitHash)
			So(err, ShouldBeNil)

			// No DEPS roll returns nil, but next should be equal to start
			So(ranges, ShouldBeNil)

			So(next.Main.RepositoryUrl, ShouldEqual, chromium)
			So(next.Main.GitHash, ShouldEqual, startGitHash)
		})

		sampleDeps2 := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': '3',
}
deps = {
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
}
	`

		Convey(`DEPS roll`, func() {
			startGitHash := "1"
			endGitHash := "2"
			resp := []*vcsinfo.LongCommit{
				{
					ShortCommit: &vcsinfo.ShortCommit{
						Hash: "2",
					},
				},
			}

			gc := &mocks.GitilesRepo{}
			gc.On("LogLinear", testutils.AnyContext, startGitHash, endGitHash).Return(resp, nil)

			gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", startGitHash).Return([]byte(sampleDeps), nil)
			gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", endGitHash).Return([]byte(sampleDeps2), nil)

			wStartGitHash := "1"
			wEndGitHash := "3"
			wResp := generateCommitResponse(3)

			wgc := &mocks.GitilesRepo{}
			wgc.On("LogLinear", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

			c := mockhttpclient.NewURLMock().Client()
			r := New(ctx, c).WithRepo(chromium, gc).WithRepo(webrtc, wgc)
			next, ranges, err := r.DetermineNextCandidate(ctx, chromium, startGitHash, endGitHash)
			So(err, ShouldBeNil)

			// Base Chromium that should be built is using startGitHash.
			So(next.Main.RepositoryUrl, ShouldEqual, chromium)
			So(next.Main.GitHash, ShouldEqual, startGitHash)

			overrides := next.ModifiedDeps[0].GitHash
			// // Next candidate should be 2, since LogLinear returns [3, 2, 1],
			// // 3 is popped leaving [2, 1]. This is reversed to [1, 2]
			// // and len()/2 = idx 1, which is commit "2"
			So(overrides, ShouldEqual, "2")

			left := ranges.Left
			So(left.Main.RepositoryUrl, ShouldEqual, chromium)
			So(left.Main.GitHash, ShouldEqual, startGitHash)
			leftDeps := left.ModifiedDeps
			So(len(leftDeps), ShouldEqual, 1)
			So(leftDeps[0].RepositoryUrl, ShouldEqual, webrtc)
			So(leftDeps[0].GitHash, ShouldEqual, wStartGitHash)

			right := ranges.Right
			So(right.Main.RepositoryUrl, ShouldEqual, chromium)
			So(right.Main.GitHash, ShouldEqual, startGitHash)
			rightDeps := right.ModifiedDeps
			So(len(rightDeps), ShouldEqual, 1)
			So(rightDeps[0].RepositoryUrl, ShouldEqual, webrtc)
			So(rightDeps[0].GitHash, ShouldEqual, wEndGitHash)
		})
	})
}
