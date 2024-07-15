package midpoint

import (
	"context"
	"strconv"
	"testing"

	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/pinpoint/go/common"
	pb "go.skia.org/infra/pinpoint/proto/v1"
)

const (
	webrtcUrl = "https://webrtc.googlesource.com/src"
	v8Url     = "https://chromium.googlesource.com/v8/v8"
)

// generateCommitResponse will create a LongCommit slice response for gitiles.Repo.LogFirstParent.
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

func assertModifiedDepsEqual(t *testing.T, expected, target []*pb.Commit) {
	require.NotNil(t, expected)
	require.NotNil(t, target)
	require.Equal(t, len(expected), len(target))

	for i, firstDep := range expected {
		secondDep := target[i]

		assert.Equal(t, firstDep.Repository, secondDep.Repository)
		assert.Equal(t, firstDep.GitHash, secondDep.GitHash)
	}
}

func TestNewCombinedCommit_WithDeps_ReturnCombinedCommit(t *testing.T) {
	main := common.NewChromiumCommit("1")
	webrtc := &pb.Commit{
		Repository: "webrtc",
		GitHash:    "2",
	}
	v8 := &pb.Commit{
		Repository: "v8",
		GitHash:    "3",
	}

	cc := common.NewCombinedCommit(main, webrtc, v8)
	assert.Equal(t, "1", cc.Main.GitHash)
	assert.Equal(t, 2, len(cc.ModifiedDeps))
}

func TestNewRepo_WithUrl_CreateNewRepo(t *testing.T) {
	ctx := context.Background()
	c := mockhttpclient.NewURLMock().Client()
	r := New(ctx, c)

	repo := r.getOrCreateRepo(v8Url)
	assert.NotNil(t, repo)
	assert.NotNil(t, r.repos[v8Url])
}

func TestFetchGitDeps_ExcludingCIPDBasedDEP_ShouldReturnDEPS(t *testing.T) {
	ctx := context.Background()

	const gitHash = "1"

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
	gc := &mocks.GitilesRepo{}
	gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", gitHash).Return([]byte(sampleDeps), nil)

	c := mockhttpclient.NewURLMock().Client()
	r := New(ctx, c).WithRepo(ChromiumSrcGit, gc)

	res, err := r.fetchGitDeps(ctx, &pb.Commit{Repository: ChromiumSrcGit, GitHash: gitHash})
	require.NoError(t, err)
	// intellij should be missing
	assert.Equal(t, 3, len(res))
	assert.Equal(t, "1", res[v8Url].GitHash)
	assert.Equal(t, "9dfa55d", res["https://chromium.googlesource.com/deps/lighttpd"].GitHash)
	assert.Equal(t, "deadbeef", res[webrtcUrl].GitHash)
}

func TestFindDepsCommit_OnExistingRepo_ShouldReturnCommit(t *testing.T) {
	ctx := context.Background()
	c := &pb.Commit{
		GitHash:    "fake-hash",
		Repository: "fake-url",
	}

	fakeDEPS := `
deps = {
	'path/to/dep': {
		'url': 'fake-dep.com@fake-dep-hash',
	},
}
`

	gr := &mocks.GitilesRepo{}
	gr.On("ReadFileAtRef", testutils.AnyContext, "DEPS", "fake-hash").Return([]byte(fakeDEPS), nil)

	m := New(ctx, nil).WithRepo("fake-url", gr)
	dc, err := m.findDepsCommit(ctx, c, "https://fake-dep.com")
	require.Nil(t, err, err)
	require.Equal(t, dc, &pb.Commit{
		GitHash:    "fake-dep-hash",
		Repository: "https://fake-dep.com",
	})
}

func TestFindDepsCommit_OnNonExistingRepo_ShouldReturnError(t *testing.T) {
	ctx := context.Background()
	c := &pb.Commit{
		GitHash:    "fake-hash",
		Repository: "fake-url",
	}

	gr := &mocks.GitilesRepo{}
	gr.On("ReadFileAtRef", testutils.AnyContext, "DEPS", "fake-hash").Return([]byte(""), nil)

	m := New(ctx, nil).WithRepo("fake-url", gr)
	dc, err := m.findDepsCommit(ctx, c, "https://some-url.com")
	require.Nil(t, dc)
	require.ErrorContains(t, err, "https://some-url.com doesn't exist in DEPS")
}

func TestCombinedCommitKey_MainNil_ReturnsEmptyString(t *testing.T) {
	cc := &common.CombinedCommit{}

	expected := cc.Clone().Key()
	assert.Equal(t, expected, cc.Key())
}

func TestCombinedCommitKey_GivenDEPS_ReturnsCombinedString(t *testing.T) {
	cc := common.NewCombinedCommit(&pb.Commit{GitHash: "hash1"}, &pb.Commit{GitHash: "hash2"}, &pb.Commit{GitHash: "hash3"})

	expected := cc.Clone().Key()
	assert.Equal(t, expected, cc.Key())
}

func TestFindMidCombinedCommit_NoModifiedDeps_ValidMidpointFromMain(t *testing.T) {
	ctx := context.Background()

	startGitHash := "1"
	endGitHash := "5"

	gc := &mocks.GitilesRepo{}
	resp := generateCommitResponse(5)
	gc.On("LogFirstParent", testutils.AnyContext, startGitHash, endGitHash).Return(resp, nil)

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(ChromiumSrcGit, gc)

	start := common.NewCombinedCommit(common.NewChromiumCommit(startGitHash))
	end := common.NewCombinedCommit(common.NewChromiumCommit(endGitHash))

	res, err := m.FindMidCombinedCommit(ctx, start, end)
	require.NoError(t, err)
	// endGitHash is popped off, leaving [4, 3, 2, 1]
	// and since len == 4, mid index == 2
	assert.Equal(t, "2", res.Main.GitHash)
}

func TestFindMidCombinedCommit_AdjacentChangesWithNoDeps_ValidMidpointFromDeps(t *testing.T) {
	ctx := context.Background()

	startGitHash := "1"
	endGitHash := "2"

	mainResp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}

	// mocks for chromium, which will be adjacent.
	gc := &mocks.GitilesRepo{}
	gc.On("LogFirstParent", testutils.AnyContext, startGitHash, endGitHash).Return(mainResp, nil)

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

	sampleDeps2 := `
vars = {
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': '3',
}
deps = {
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
}
	`

	gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", startGitHash).Return([]byte(sampleDeps), nil)
	gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", endGitHash).Return([]byte(sampleDeps2), nil)

	// mocks for webrtc, which is parsed as a delta from DEPS
	wStartGitHash := "1"
	wEndGitHash := "3"
	wResp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "3",
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}

	wgc := &mocks.GitilesRepo{}
	wgc.On("LogFirstParent", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(ChromiumSrcGit, gc).WithRepo(webrtcUrl, wgc)

	start := common.NewCombinedCommit(common.NewChromiumCommit(startGitHash))
	end := common.NewCombinedCommit(common.NewChromiumCommit(endGitHash))

	// no modified deps in start and end, meaning we go through the regular workflow of searching
	// for midpoint in chromium.
	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)
	// Next candidate should be 2, since LogFirstParent returns [3, 2],
	// 3 is popped leaving [2].
	nextCommit := res.GetLatestModifiedDep()
	assert.Equal(t, "2", nextCommit.GitHash)
}

func TestFindMidCombinedCommit_WithModifiedDeps_NextCandidateInModifiedDeps(t *testing.T) {
	ctx := context.Background()
	wStartGitHash := "1"
	wEndGitHash := "5"

	wgc := &mocks.GitilesRepo{}
	wResp := generateCommitResponse(5)
	wgc.On("LogFirstParent", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

	start := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wStartGitHash,
			Repository: webrtcUrl,
		})
	end := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wEndGitHash,
			Repository: webrtcUrl,
		})

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(webrtcUrl, wgc)
	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)
	// endGitHash is popped off, leaving [4, 3, 2, 1]
	// and since len == 4, mid index == 2
	nextCommit := res.GetLatestModifiedDep()
	assert.Equal(t, "2", nextCommit.GitHash)
}

func TestFindMidCombinedCommit_AdjacentModifiedDeps_NextCandidateWithinDeps(t *testing.T) {
	ctx := context.Background()
	wStartGitHash := "1"
	wEndGitHash := "2"

	// Test prep for webrtc mock.
	wgc := &mocks.GitilesRepo{}
	wResp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}
	wgc.On("LogFirstParent", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

	sampleDeps := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '1',
}
  `
	sampleDeps2 := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '3',
}
	`
	wgc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wStartGitHash).Return([]byte(sampleDeps), nil)
	wgc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wEndGitHash).Return([]byte(sampleDeps2), nil)

	// Test prep for v8 mock, which should be invoked after webrtc deps are parsed.
	v8gc := &mocks.GitilesRepo{}
	v8resp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "3",
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}
	v8gc.On("LogFirstParent", testutils.AnyContext, "1", "3").Return(v8resp, nil)

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(webrtcUrl, wgc).WithRepo(v8Url, v8gc)

	start := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wStartGitHash,
			Repository: webrtcUrl,
		})
	end := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wEndGitHash,
			Repository: webrtcUrl,
		})

	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)

	// The next candidate should be in v8/v8 with commit "2", because the midpoint
	// from [3, 2] would be 2.
	nextCommit := res.GetLatestModifiedDep()
	assert.Equal(t, v8Url, nextCommit.Repository)
	assert.Equal(t, "2", nextCommit.GitHash)
}

func TestFindMidCombinedCommit_AdjacentModifiedDeps_NoMoreCandidates(t *testing.T) {
	ctx := context.Background()
	wStartGitHash := "1"
	wEndGitHash := "2"

	// Test prep for webrtc repository mocks
	wgc := &mocks.GitilesRepo{}
	wResp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}
	wgc.On("LogFirstParent", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

	sampleDeps := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '3',
}
  `
	sampleDeps2 := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '4',
}
	`
	wgc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wStartGitHash).Return([]byte(sampleDeps), nil)
	wgc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wEndGitHash).Return([]byte(sampleDeps2), nil)

	// Test prep for v8/v8. Midpoint should be checked after webrtc DEPS files are parsed.
	v8gc := &mocks.GitilesRepo{}
	v8resp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "4",
			},
		},
	}
	// If the roll was only to a n+1 commit, they are adjacent and we terminate saying there's
	// no more candidates to go through.
	v8gc.On("LogFirstParent", testutils.AnyContext, "3", "4").Return(v8resp, nil)

	start := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wStartGitHash,
			Repository: webrtcUrl,
		})
	end := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wEndGitHash,
			Repository: webrtcUrl,
		})

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(webrtcUrl, wgc).WithRepo(v8Url, v8gc)
	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)
	// equality check through key
	assert.Equal(t, start.Key(), res.Key())
}

func TestFindMidCombinedCommit_StartIsLastCommitInDEPS_ReturnsStartAsMidpoint(t *testing.T) {
	ctx := context.Background()
	wStartGitHash := "1"
	wEndGitHash := "2"

	// Test prep for webrtc repository mocks
	wgc := &mocks.GitilesRepo{}
	wResp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}
	wgc.On("LogFirstParent", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

	sampleDeps := `
vars = {
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': '1',
}
deps = {
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
}
	`
	// This should be invoked as it fills modified deps for the end commit.
	gc := &mocks.GitilesRepo{}
	gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wEndGitHash).Return([]byte(sampleDeps), nil)

	start := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wStartGitHash,
			Repository: webrtcUrl,
		})
	end := common.NewCombinedCommit(common.NewChromiumCommit(wEndGitHash))

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(ChromiumSrcGit, gc).WithRepo(webrtcUrl, wgc)
	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)
	assert.Equal(t, start.Key(), res.Key())
}

func TestFindMidCombinedCommit_EndIsFirstCommitInDEPS_ReturnsStartAsMidpoint(t *testing.T) {
	ctx := context.Background()
	wStartGitHash := "1"
	wEndGitHash := "1"

	// Test prep for webrtc repository mocks
	wgc := &mocks.GitilesRepo{}
	wResp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}
	wgc.On("LogFirstParent", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

	sampleDeps := `
vars = {
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': '1',
}
deps = {
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
}
	`
	// This should be invoked as it fills modified deps for the end commit.
	gc := &mocks.GitilesRepo{}
	gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wEndGitHash).Return([]byte(sampleDeps), nil)

	start := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash))
	end := common.NewCombinedCommit(common.NewChromiumCommit(wEndGitHash),
		&pb.Commit{
			GitHash:    wEndGitHash,
			Repository: webrtcUrl,
		})
	origStart := start.Clone()

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(ChromiumSrcGit, gc).WithRepo(webrtcUrl, wgc)
	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)
	assert.Equal(t, origStart.Key(), res.Key())
}

func TestFindMidCombinedCommit_ComparisonWithUnbalancedModifiedDeps_ValidNextCandidate(t *testing.T) {
	ctx := context.Background()
	wStartGitHash := "1"
	wEndGitHash := "5"

	wgc := &mocks.GitilesRepo{}
	wResp := generateCommitResponse(5)
	wgc.On("LogFirstParent", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

	sampleDeps := `
vars = {
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': '1',
}
deps = {
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
}
	`
	// This should be invoked as it fills modified deps for the start commit.
	gc := &mocks.GitilesRepo{}
	gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wStartGitHash).Return([]byte(sampleDeps), nil)

	start := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash))
	end := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wEndGitHash,
			Repository: webrtcUrl,
		})

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(ChromiumSrcGit, gc).WithRepo(webrtcUrl, wgc)
	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)
	// endGitHash is popped off, leaving [4, 3, 2, 1]
	nextCommit := res.GetLatestModifiedDep()
	assert.Equal(t, "2", nextCommit.GitHash)
}

func TestFindMidCombinedCommit_ComparisonWithMultipleModifiedDepsAdjacent_DepsWithinDepsMidpoint(t *testing.T) {
	ctx := context.Background()
	chromiumStartGitHash := "1"

	wgc := &mocks.GitilesRepo{}
	sampleDeps := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '2',
}
	`
	// Start has V8, and end doesn't, meaning that end needs to be backfilled.
	// Webrtc@2 should be used as reference to backfill.
	wgc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", "2").Return([]byte(sampleDeps), nil)

	v8gc := &mocks.GitilesRepo{}
	v8Resp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}
	v8gc.On("LogFirstParent", testutils.AnyContext, "1", "2").Return(v8Resp, nil)

	v8sampleDeps1 := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
}
deps = {
  'random': Var('chromium_git') + '/random.git' + '@' + '3',
}
	`
	v8sampleDeps2 := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
}
deps = {
  'random': Var('chromium_git') + '/random.git' + '@' + '5',
}
	`
	// Since V8 is also adjacent, it assumes a DEPS roll and parses one level deeper into DEPS.
	v8gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", "1").Return([]byte(v8sampleDeps1), nil)
	v8gc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", "2").Return([]byte(v8sampleDeps2), nil)

	randomUrl := "https://chromium.googlesource.com/random"
	randomGc := &mocks.GitilesRepo{}
	randomGcResp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "5",
			},
		},
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "4",
			},
		},
	}
	randomGc.On("LogFirstParent", testutils.AnyContext, "3", "5").Return(randomGcResp, nil)
	start := common.NewCombinedCommit(common.NewChromiumCommit(chromiumStartGitHash),
		&pb.Commit{
			GitHash:    "1",
			Repository: webrtcUrl,
		},
		&pb.Commit{
			GitHash:    "1",
			Repository: v8Url,
		},
	)
	end := common.NewCombinedCommit(common.NewChromiumCommit(chromiumStartGitHash),
		&pb.Commit{
			GitHash:    "2",
			Repository: webrtcUrl,
		},
	)

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(v8Url, v8gc).WithRepo(webrtcUrl, wgc).WithRepo(randomUrl, randomGc)
	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)
	// [5, 4], top is popped off so midpoint = 4
	nextCommit := res.GetLatestModifiedDep()
	assert.Equal(t, "4", nextCommit.GitHash)
}

func TestFindMidCombinedCommit_DEPSFileDoesNotExist_NoMidpoint(t *testing.T) {
	ctx := context.Background()
	wStartGitHash := "1"
	wEndGitHash := "2"

	// Test prep for webrtc mock.
	wgc := &mocks.GitilesRepo{}
	wResp := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: "2",
			},
		},
	}
	wgc.On("LogFirstParent", testutils.AnyContext, wStartGitHash, wEndGitHash).Return(wResp, nil)

	sampleDeps := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '1',
}
  `
	wgc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wStartGitHash).Return([]byte(sampleDeps), nil)
	wgc.On("ReadFileAtRef", testutils.AnyContext, "DEPS", wEndGitHash).Return(nil, skerr.Fmt("Request got status \"404 Not Found\""))

	start := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wStartGitHash,
			Repository: webrtcUrl,
		},
	)
	end := common.NewCombinedCommit(common.NewChromiumCommit(wStartGitHash),
		&pb.Commit{
			GitHash:    wEndGitHash,
			Repository: webrtcUrl,
		},
	)

	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(webrtcUrl, wgc)
	res, err := m.FindMidCombinedCommit(ctx, start, end)
	assert.NoError(t, err)
	assert.Equal(t, start.Key(), res.Key())
}

func TestFillModifiedDeps_EmptyEndCommitModifiedDeps(t *testing.T) {
	startGitHash := "1"
	start := common.NewCombinedCommit(common.NewChromiumCommit(startGitHash),
		&pb.Commit{
			GitHash:    startGitHash,
			Repository: webrtcUrl,
		},
		&pb.Commit{
			GitHash:    startGitHash,
			Repository: v8Url,
		},
	)

	endCommitDeps := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': '2',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '2',
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
}
  `
	end := common.NewCombinedCommit(common.NewChromiumCommit(startGitHash))

	chromiumClient := &mocks.GitilesRepo{}
	chromiumClient.On("ReadFileAtRef", testutils.AnyContext, "DEPS", startGitHash).Return([]byte(endCommitDeps), nil)

	webrtcClient := &mocks.GitilesRepo{}
	webrtcClient.On("ReadFileAtRef", testutils.AnyContext, "DEPS", "2").Return([]byte(endCommitDeps), nil)

	ctx := context.Background()
	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(ChromiumSrcGit, chromiumClient).WithRepo(webrtcUrl, webrtcClient)

	err := m.fillModifiedDeps(ctx, start, end)
	require.NoError(t, err)
	expected := []*pb.Commit{
		{
			Repository: webrtcUrl,
			GitHash:    "2",
		},
		{
			Repository: v8Url,
			GitHash:    "2",
		},
	}
	assertModifiedDepsEqual(t, expected, end.ModifiedDeps)
}

func TestFillModifiedDeps_EmptyStartCommitModifiedDeps(t *testing.T) {
	startGitHash := "1"
	startCommitDeps := `
vars = {
  'chromium_git': 'https://chromium.googlesource.com',
  'webrtc_git': 'https://webrtc.googlesource.com',
  'webrtc_rev': '2',
}
deps = {
  'src/v8': Var('chromium_git') + '/v8/v8.git' + '@' + '2',
  'src/third_party/webrtc': {
    'url': '{webrtc_git}/src.git@{webrtc_rev}',
  },
}
  `
	start := common.NewCombinedCommit(common.NewChromiumCommit(startGitHash))

	end := common.NewCombinedCommit(common.NewChromiumCommit(startGitHash),
		&pb.Commit{
			GitHash:    startGitHash,
			Repository: webrtcUrl,
		},
		&pb.Commit{
			GitHash:    startGitHash,
			Repository: v8Url,
		},
	)

	chromiumClient := &mocks.GitilesRepo{}
	chromiumClient.On("ReadFileAtRef", testutils.AnyContext, "DEPS", startGitHash).Return([]byte(startCommitDeps), nil)

	webrtcClient := &mocks.GitilesRepo{}
	webrtcClient.On("ReadFileAtRef", testutils.AnyContext, "DEPS", "2").Return([]byte(startCommitDeps), nil)

	ctx := context.Background()
	c := mockhttpclient.NewURLMock().Client()
	m := New(ctx, c).WithRepo(ChromiumSrcGit, chromiumClient).WithRepo(webrtcUrl, webrtcClient)

	err := m.fillModifiedDeps(ctx, start, end)
	require.NoError(t, err)
	expected := []*pb.Commit{
		{
			Repository: webrtcUrl,
			GitHash:    "2",
		},
		{
			Repository: v8Url,
			GitHash:    "2",
		},
	}
	assertModifiedDepsEqual(t, expected, start.ModifiedDeps)
}
