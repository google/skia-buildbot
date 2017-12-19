package failures

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	swarmingv1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/suggester/go/dsconst"
)

func TestAdd(t *testing.T) {
	f := Failures{}
	f.Add("include/core/SkColorSpace.h", "Bot-1")
	f.Add("include/core/SkColorSpace.h ", "Bot-2")
	f.Add("  include/core/SkColorSpace.h", "Bot-2")
	f.Add("include/core/SkRect.h", "Bot-2")
	f.Add("/COMMIT_MSG", "Bot-2")

	assert.Equal(t, 2, f["include/core/SkColorSpace.h"]["Bot-2"])
	assert.Equal(t, 1, f["include/core/SkColorSpace.h"]["Bot-1"])
	assert.Equal(t, 0, f["include/core/SkColorSpace.h"]["unknown bot"])
	assert.Equal(t, 0, f["/COMMIT_MSG"]["Bot-2"])

	assert.Equal(t, 3, f["include"]["Bot-2"])
	assert.Equal(t, 3, f["include/core"]["Bot-2"])
}

var (
	hash1 = ""
	hash2 = ""
	hash3 = ""
)

func badbot(botname string, ts time.Time) bool {
	return botname == "bot-bad"
}

func taskListProvider(since time.Duration) ([]*swarmingv1.SwarmingRpcsTaskRequestMetadata, error) {
	now := time.Now()
	nowStr := now.Format(time.RFC3339Nano)
	return []*swarmingv1.SwarmingRpcsTaskRequestMetadata{
		&swarmingv1.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarmingv1.SwarmingRpcsTaskResult{
				Tags: []string{
					"sk_issue_server:https://skia-review.googlesource.com",
					"sk_issue:82041",
					"sk_patchset:1",
					"sk_name:Test-Win10-Clang-Golo-GPU-QuadroP400-x86_64-Release-All-Vulkan",
					"sk_repo:https://skia.googlesource.com/skia.git",
				},
				StartedTs: nowStr,
			},
		},
		&swarmingv1.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarmingv1.SwarmingRpcsTaskResult{
				Tags: []string{
					fmt.Sprintf("sk_revision:%s", hash1),
					"sk_name:Test-Win10-Clang-Golo-GPU-QuadroP400-x86_64-Debug-All-Vulkan",
					"sk_repo:https://skia.googlesource.com/skia.git",
				},
				StartedTs: nowStr,
			},
		},
	}, nil
}

func TestStore(t *testing.T) {
	testutils.MediumTest(t)

	now := time.Now()

	cleanup := testutil.InitDatastore(t, dsconst.FLAKY_RANGES)
	defer cleanup()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `)]}'\n{"somefile.txt":{}}`)
	}))
	defer ts.Close()

	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	defer g.Cleanup()

	hash1 = g.CommitGen(ctx, "README.md")
	hash2 = g.CommitGen(ctx, "README.md")
	hash3 = g.CommitGen(ctx, "INSTALL.md")
	git := &git.Checkout{git.GitDir(g.Dir())}
	fs := New(badbot, taskListProvider, ds.DS, git, ts.Client(), "https://skia.googlesource.com/skia.git")
	f, err := fs.List(now.Add(-1*time.Hour), now)
	assert.NoError(t, err)
	assert.Len(t, f, 0)

	err = fs.Update(time.Hour)
	assert.NoError(t, err)

	f, err = fs.List(now.Add(-1*time.Hour), now)
	assert.NoError(t, err)
	assert.Len(t, f, 2)
}
