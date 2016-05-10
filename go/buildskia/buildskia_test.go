package buildskia

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/util"
)

func TestGetSkiaHash(t *testing.T) {
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
	deps := ``
	body := base64.StdEncoding.EncodeToString([]byte(deps))
	client := mockhttpclient.New(map[string]mockhttpclient.MockDialogue{
		CHROMIUM_DEPS_URL: mockhttpclient.MockGetDialogue([]byte(body)),
	})

	_, err := GetSkiaHash(client)
	assert.Error(t, err)
}

func TestGetSkiaBranches(t *testing.T) {
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
	for branch, _ := range br {
		keys = append(keys, branch)
	}
	assert.True(t, util.In("refs/heads/chrome/m50", keys))
}

func TestGetSkiaBranchesEmpty(t *testing.T) {
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
