package midpoint

import (
	"context"
	"strconv"
	"testing"

	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewRepo_WithUrl_CreateNewRepo(t *testing.T) {
	ctx := context.Background()
	c := mockhttpclient.NewURLMock().Client()
	r := New(ctx, c)

	const url = "https://somerepo.com"
	repo := r.getOrCreateRepo(url)
	assert.NotNil(t, repo)
	assert.NotNil(t, r.repos[url])
}

func TestFetchGitDeps_ExcludingCIPDBasedDEP_ShouldReturnDEPS(t *testing.T) {
	ctx := context.Background()

	const chromium = "https://chromium.org/chromium/src"
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
	r := New(ctx, c).WithRepo(chromium, gc)

	res, err := r.fetchGitDeps(ctx, gc, gitHash)
	require.NoError(t, err)
	// intellij should be missing
	assert.Equal(t, 3, len(res))
	assert.Equal(t, "1", res["https://chromium.googlesource.com/v8/v8"])
	assert.Equal(t, "9dfa55d", res["https://chromium.googlesource.com/deps/lighttpd"])
	assert.Equal(t, "deadbeef", res["https://webrtc.googlesource.com/src"])
}

func TestFindDepsCommit_OnExistingRepo_ShouldReturnCommit(t *testing.T) {
	ctx := context.Background()
	c := &Commit{
		GitHash:       "fake-hash",
		RepositoryUrl: "fake-url",
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
	dc, err := m.FindDepsCommit(ctx, c, "https://fake-dep.com")
	require.Nil(t, err, err)
	require.Equal(t, dc, &Commit{
		GitHash:       "fake-dep-hash",
		RepositoryUrl: "https://fake-dep.com",
	})
}

func TestFindDepsCommit_OnNonExistingRepo_ShouldReturnError(t *testing.T) {
	ctx := context.Background()
	c := &Commit{
		GitHash:       "fake-hash",
		RepositoryUrl: "fake-url",
	}

	gr := &mocks.GitilesRepo{}
	gr.On("ReadFileAtRef", testutils.AnyContext, "DEPS", "fake-hash").Return([]byte(""), nil)

	m := New(ctx, nil).WithRepo("fake-url", gr)
	dc, err := m.FindDepsCommit(ctx, c, "https://some-url.com")
	require.Nil(t, dc)
	require.ErrorContains(t, err, "https://some-url.com doesn't exist in DEPS")
}

func TestCombinedCommitKey_MainNil_ReturnsEmptyString(t *testing.T) {
	cc := &CombinedCommit{}
	assert.Empty(t, cc.Key())
}

func TestCombinedCommitKey_GivenDEPS_ReturnsCombinedString(t *testing.T) {
	cc := &CombinedCommit{
		Main: &Commit{
			GitHash: "hash1",
		},
		ModifiedDeps: []*Commit{
			{GitHash: "hash2"}, {GitHash: "hash3"},
		},
	}
	assert.Equal(t, cc.Key(), "hash1+hash2+hash3")
}
