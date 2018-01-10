package failures

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	swarmingv1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/predict/go/dsconst"
	httpmock "gopkg.in/jarcoal/httpmock.v1"
)

func TestAddPredict(t *testing.T) {
	testutils.SmallTest(t)
	f := Failures{}
	f.Add("include/core/SkColorSpace.h", "Bot-1")
	f.Add("include/core/SkColorSpace.h ", "Bot-2")
	f.Add("  include/core/SkColorSpace.h", "Bot-2")
	f.Add("include/core/SkRect.h", "Bot-2")
	f.Add("include/core/SkOval.h", "Bot-3")
	f.Add("/COMMIT_MSG", "Bot-2")
	f.Add("include/utils/SkHelper.h", "Bot-5")

	assert.Equal(t, 2, f["include/core/SkColorSpace.h"]["Bot-2"])
	assert.Equal(t, 1, f["include/core/SkColorSpace.h"]["Bot-1"])
	assert.Equal(t, 0, f["include/core/SkColorSpace.h"]["unknown bot"])
	assert.Equal(t, 0, f["/COMMIT_MSG"]["Bot-2"])

	assert.Equal(t, 3, f["include"]["Bot-2"])
	assert.Equal(t, 3, f["include/core"]["Bot-2"])

	p := f.Predict([]string{"include/core/SkColorSpace.h"})
	assert.Equal(t, []Summary{
		{"Bot-2", 2},
		{"Bot-1", 1},
	}, p)
	p = f.Predict([]string{"include/core/some-other-file-in-core.h"})
	assert.Equal(t, []Summary{
		{"Bot-2", 3},
		{"Bot-1", 1},
		{"Bot-3", 1},
	}, p)
	p = f.Predict([]string{"include/gpu/GrSomething.h"})
	assert.Equal(t, []Summary{
		{"Bot-2", 3},
		{"Bot-1", 1},
		{"Bot-3", 1},
		{"Bot-5", 1},
	}, p)
	p = f.Predict([]string{"src/core/GrSomething.cpp"})
	assert.Equal(t, []Summary{}, p)
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
	now := time.Now().UTC()
	return []*swarmingv1.SwarmingRpcsTaskRequestMetadata{
		&swarmingv1.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarmingv1.SwarmingRpcsTaskResult{
				Tags: []string{
					"sk_issue_server:https://skia-review.googlesource.com",
					"sk_issue:82041",
					"sk_patchset:1",
					"sk_name:Test-Win10",
					"sk_repo:https://skia.googlesource.com/skia.git",
				},
				StartedTs: now.Add(-10 * time.Minute).Format(TS_FORMAT),
			},
		},
		&swarmingv1.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarmingv1.SwarmingRpcsTaskResult{
				Tags: []string{
					fmt.Sprintf("sk_revision:%s", hash1),
					"sk_name:Test-Linux",
					"sk_repo:https://skia.googlesource.com/skia.git",
				},
				StartedTs: now.Add(-1 * time.Minute).Format(TS_FORMAT),
			},
		},
		// The following should be ignored.
		&swarmingv1.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarmingv1.SwarmingRpcsTaskResult{
				Tags: []string{
					"sk_revision:blahblahblah", // Unknown git hash.
					"sk_name:Test-Linux",
					"sk_repo:https://skia.googlesource.com/skia.git",
				},
				StartedTs: now.Add(-1 * time.Minute).Format(TS_FORMAT),
			},
		},
		&swarmingv1.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarmingv1.SwarmingRpcsTaskResult{
				Tags: []string{
					fmt.Sprintf("sk_revision:%s", hash1),
					"sk_name:bot-bad", // Should be filtered out by badbot().
					"sk_repo:https://skia.googlesource.com/skia.git",
				},
				StartedTs: now.Add(-1 * time.Minute).Format(TS_FORMAT),
			},
		},
		&swarmingv1.SwarmingRpcsTaskRequestMetadata{
			TaskResult: &swarmingv1.SwarmingRpcsTaskResult{
				Tags: []string{
					fmt.Sprintf("sk_revision:%s", hash1),
					"sk_name:Upload-Some-Test-Results", // Should be filtered out since it's an upload task.
					"sk_repo:https://skia.googlesource.com/skia.git",
				},
				StartedTs: now.Add(-1 * time.Minute).Format(TS_FORMAT),
			},
		},
	}, nil
}

func TestStore(t *testing.T) {
	testutils.MediumTest(t)

	now := time.Now()

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://skia-review.googlesource.com/changes/82041/revisions/1/files/",
		httpmock.NewStringResponder(200, `)]}' {"somefile.txt":{}}`))

	cleanup := testutil.InitDatastore(t, dsconst.FLAKY_RANGES)
	defer cleanup()

	ctx := context.Background()
	g := git_testutils.GitInit(t, ctx)
	defer g.Cleanup()

	hash1 = g.CommitGen(ctx, "README.md")
	hash2 = g.CommitGen(ctx, "README.md")
	hash3 = g.CommitGen(ctx, "INSTALL.md")
	git := &git.Checkout{
		GitDir: git.GitDir(g.Dir()),
	}
	fs := New(badbot, taskListProvider, git, http.DefaultClient, "https://skia.googlesource.com/skia.git")
	f, err := fs.List(now.Add(-1*time.Hour), now)
	assert.NoError(t, err)
	assert.Len(t, f, 0)

	err = fs.Update(time.Hour)
	assert.NoError(t, err)

	f, err = fs.List(now.Add(-1*time.Hour), now)
	assert.NoError(t, err)
	assert.Len(t, f, 2)
	assert.Equal(t, "Test-Win10", f[0].BotName)
	assert.Equal(t, []string{"somefile.txt"}, f[0].Files)

	assert.Equal(t, "Test-Linux", f[1].BotName)
	assert.Equal(t, []string{"README.md"}, f[1].Files)
}
