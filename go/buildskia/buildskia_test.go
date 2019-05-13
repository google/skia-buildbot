package buildskia

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

func TestGetSkiaHash(t *testing.T) {
	unittest.SmallTest(t)
	deps := `vars = {
    # Use this googlecode_url variable only if there is an internal mirror for it.
    # If you do not know, use the full path while defining your new deps entry.
    'googlecode_url': 'http://%s.googlecode.com/svn',
    'chromium_git': 'https://chromium.googlesource.com',
    # Three lines of non-changing comments so that
    # the commit queue can handle CLs rolling sfntly
    # and whatever else without interference from each other.
    'sfntly_revision': '130f832eddf98467e6578b548cb74ce17d04a26d',
    # Three lines of non-changing comments so that
    # the commit queue can handle CLs rolling Skia
    # and whatever else without interference from each other.
    'skia_revision': '142659c76dfca1e0a34eb6a022329b73b6ba3166',
    # Three lines of non-changing comments so that
    # the commit queue can handle CLs rolling V8
    # and whatever else without interference from each other.
    'v8_revision': 'edb7ef701c169a11a69e2be028534936ffb56346',
  `
	body := base64.StdEncoding.EncodeToString([]byte(deps))
	client := mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		CHROMIUM_DEPS_URL: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	hash, err := GetSkiaHash(client)
	assert.NoError(t, err)
	assert.Equal(t, "142659c76dfca1e0a34eb6a022329b73b6ba3166", hash)
}

func TestGetSkiaHashEmpty(t *testing.T) {
	unittest.SmallTest(t)
	deps := ``
	body := base64.StdEncoding.EncodeToString([]byte(deps))
	client := mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		CHROMIUM_DEPS_URL: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	_, err := GetSkiaHash(client)
	assert.Error(t, err)
}

func TestGetSkiaBranches(t *testing.T) {
	unittest.SmallTest(t)
	body := `)]}'
{
  "HEAD": {
    "value": "142659c76dfca1e0a34eb6a022329b73b6ba3166",
    "target": "refs/heads/master"
  },
  "refs/branch-heads/m20_1132": {
    "value": "c66137cb834d35c9b403fe81dd1700396ea7b056"
  },
  "refs/heads/chrome/m49": {
    "value": "e2913ed9b25bf4a47194c4ca134beec0b5784842"
  },
  "refs/heads/chrome/m50": {
    "value": "dde87ad6d5278661aac6a8eda9e8f43deb255fe2"
  },
  "refs/heads/infra/config": {
    "value": "16de5a78b524795d9e8f619be3fe96d6b82dd397"
  },
  "refs/heads/master": {
    "value": "142659c76dfca1e0a34eb6a022329b73b6ba3166"
  },
  "refs/internal/git-svn-max-branch-rev": {
    "value": "8f0ed522970c2ea01050379a12be5d5e58632e66"
  }
}`
	client := mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		SKIA_BRANCHES_JSON: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	br, err := GetSkiaBranches(client)
	assert.NoError(t, err)
	assert.Equal(t, 7, len(br))
	keys := []string{}
	for branch := range br {
		keys = append(keys, branch)
	}
	assert.True(t, util.In("refs/heads/chrome/m50", keys))
}

func TestGetSkiaBranchesEmpty(t *testing.T) {
	unittest.SmallTest(t)
	body := `)]}'`
	client := mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		SKIA_BRANCHES_JSON: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	_, err := GetSkiaBranches(client)
	assert.Error(t, err)

	body = ``
	client = mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		SKIA_BRANCHES_JSON: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	_, err = GetSkiaBranches(client)
	assert.Error(t, err)
}

func TestGetSkiaHead(t *testing.T) {
	unittest.SmallTest(t)
	body := `)]}'
{
    "commit": "273c0f5e87397c40d22bb7e3ee078bb46a3f6860",
    "tree": "70436fd146c39be9702c6c295a8fd204a38d865f",
    "parents": [
    "a5598a40f82d69113fb4764dcb8de62151921807"
    ]
}`
	client := mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		SKIA_HEAD_JSON: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	hash, err := GetSkiaHead(client)
	assert.NoError(t, err)
	assert.Equal(t, "273c0f5e87397c40d22bb7e3ee078bb46a3f6860", hash)
}

func TestGetSkiaHeadEmpty(t *testing.T) {
	unittest.SmallTest(t)
	body := `)]}'`
	client := mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		SKIA_BRANCHES_JSON: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	_, err := GetSkiaBranches(client)
	assert.Error(t, err)

	body = ``
	client = mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		SKIA_HEAD_JSON: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	_, err = GetSkiaHead(client)
	assert.Error(t, err)
}

func TestGNGen(t *testing.T) {
	unittest.SmallTest(t)
	mock := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mock.Run)

	err := GNGen(ctx, "/mnt/pd0/skia/", "/mnt/pd0/depot_tools", "Debug", []string{"is_debug=true"})
	assert.NoError(t, err)

	got, want := exec.DebugString(mock.Commands()[0]), `gn gen out/Debug --args=is_debug=true`
	if !strings.HasSuffix(got, want) {
		t.Errorf("Failed: Command %q doesn't end with %q", got, want)
	}
}

func TestGNNinjaBuild(t *testing.T) {
	unittest.SmallTest(t)
	mock := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mock.Run)

	_, err := GNNinjaBuild(ctx, "/mnt/pd0/skia/", "/mnt/pd0/depot_tools", "Debug", "", false)
	assert.NoError(t, err)
	got, want := exec.DebugString(mock.Commands()[0]), "/mnt/pd0/depot_tools/ninja -C out/Debug"
	if !strings.HasSuffix(got, want) {
		t.Errorf("Failed: Command %q doesn't end with %q", got, want)
	}
}

func TestGNDownloadSkia(t *testing.T) {
	unittest.SmallTest(t)
	mock := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mock.Run)

	checkout, err := ioutil.TempDir("", "download-test")
	assert.NoError(t, err)
	defer func() {
		err := os.RemoveAll(checkout)
		if err != nil {
			t.Logf("Failed to clean up checkout: %s", err)
		}
	}()
	err = os.MkdirAll(filepath.Join(checkout, "skia"), 0777)
	assert.NoError(t, err)

	_, err = GNDownloadSkia(ctx, "master", "aabbccddeeff", checkout, "/mnt/pd0/fiddle/depot_tools", false, false)
	// Not all of exec is mockable, so GNDownloadSkia will fail, but check the correctness
	// of the commands we did issue before hitting the failure point.
	assert.Error(t, err)
	expectedCommands := []string{
		"fetch skia",
		"git show-ref",
		"git rev-list --max-parents=0 HEAD",
		"git reset --hard aabbccddeeff",
		"gclient sync",
		"fetch-gn",
		"git log -n 1 --format=format:%H%n%P%n%an%x20(%ae)%n%s%n%b aabbccddeeff",
	}
	assert.Equal(t, len(expectedCommands), len(mock.Commands()))
	for i, want := range expectedCommands {
		got := exec.DebugString(mock.Commands()[i])
		if !strings.HasSuffix(got, want) {
			t.Errorf("Failed: Command %q doesn't end with %q", got, want)
		}
	}
}

func TestGNNinjaBuildTarget(t *testing.T) {
	unittest.SmallTest(t)
	mock := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mock.Run)

	_, err := GNNinjaBuild(ctx, "/mnt/pd0/skia/", "/mnt/pd0/depot_tools", "Debug", "fiddle", false)
	assert.NoError(t, err)
	got, want := exec.DebugString(mock.Commands()[0]), "/mnt/pd0/depot_tools/ninja -C out/Debug fiddle"
	if !strings.HasSuffix(got, want) {
		t.Errorf("Failed: Command %q doesn't end with %q", got, want)
	}
}
