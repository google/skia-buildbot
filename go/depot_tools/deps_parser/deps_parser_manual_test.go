package deps_parser

import (
	"context"
	"testing"

	"github.com/go-python/gpython/ast"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gitiles"
)

func TestParseDepsRealWorld(t *testing.T) {
	// Manual test, since it loads data from real APIs.

	type depsEntryPos struct {
		*DepsEntry
		*ast.Pos
	}
	ctx := context.Background()

	// checkDeps loads the DEPS file from the given repo at the given
	// revision and asserts that it contains the given deps.
	checkDeps := func(repo string, rev string, expectMap map[string]*depsEntryPos) {
		contents, err := gitiles.NewRepo(repo, nil).ReadFileAtRef(ctx, "DEPS", rev)
		require.NoError(t, err)
		actual, poss, err := parseDeps(string(contents))
		require.NoError(t, err)
		actualMap := make(map[string]*depsEntryPos, len(actual))
		for depId, dep := range actual {
			actualMap[depId] = &depsEntryPos{
				DepsEntry: dep,
				Pos:       poss[depId],
			}
		}
		for id, expect := range expectMap {
			assertdeep.Equal(t, expect, actualMap[id])
		}
	}

	// Chromium DEPS. We expect this to be the most complex example of a
	// DEPS file. Check a few example dependencies.
	checkDeps("https://chromium.googlesource.com/chromium/src.git", "9f0b31d5560995206ce92535935c2989913bd5bd", map[string]*depsEntryPos{
		"skia.googlesource.com/skia": {
			// Skia is a simple (non-dict) dep with a vars lookup.
			DepsEntry: &DepsEntry{
				Id:      "skia.googlesource.com/skia",
				Version: "85755f46a8810b1863493a81887f1dc17c2e49e1",
				Path:    "src/third_party/skia",
			},
			Pos: &ast.Pos{
				Lineno:    178,
				ColOffset: 19,
			},
		},
		"infra/tools/luci/swarming": {
			// Swarming client is a CIPD dep with a vars lookup.
			DepsEntry: &DepsEntry{
				Id:      "infra/tools/luci/swarming",
				Version: "git_revision:de73cf6c4bde86f0a9c8d54151b69b0154a398f1",
				Path:    "src/tools/luci-go",
			},
			Pos: &ast.Pos{
				Lineno:    159,
				ColOffset: 13,
			},
		},
		"android.googlesource.com/platform/external/protobuf": {
			// This is a dict entry whose version is defined in place.
			DepsEntry: &DepsEntry{
				Id:      "android.googlesource.com/platform/external/protobuf",
				Version: "7fca48d8ce97f7ba3ab8eea5c472f1ad3711762f",
				Path:    "src/third_party/android_protobuf/src",
			},
			Pos: &ast.Pos{
				Lineno:    640,
				ColOffset: 76,
			},
		},
	})

	// ANGLE. This entry caused a problem in the past.
	checkDeps("https://chromium.googlesource.com/angle/angle.git", "390ef29999bc0b1c1b976c1428b5914718477f4e", map[string]*depsEntryPos{
		"chromium.googlesource.com/external/github.com/KhronosGroup/SPIRV-Tools": {
			DepsEntry: &DepsEntry{
				Id:      "chromium.googlesource.com/external/github.com/KhronosGroup/SPIRV-Tools",
				Version: "e95fbfb1f509ad7a7fdfb72ac35fe412d72fc4a4",
				Path:    "third_party/spirv-tools/src",
			},
			Pos: &ast.Pos{
				Lineno:    49,
				ColOffset: 26,
			},
		},
	})

	// This DEPS file has an unpinned entry.
	checkDeps("https://chromium.googlesource.com/infra/infra.git", "fbd6fe605e30b496eab7a1ddb367cfb24cb86d99", map[string]*depsEntryPos{
		"chromium.googlesource.com/chromium/tools/build": {
			DepsEntry: &DepsEntry{
				Id:      "chromium.googlesource.com/chromium/tools/build",
				Version: "",
				Path:    "build",
			},
			Pos: &ast.Pos{
				Lineno:    8,
				ColOffset: 4,
			},
		},
	})
}
