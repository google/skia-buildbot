package deps_parser

import (
	"context"
	"strings"
	"testing"

	"github.com/go-python/gpython/ast"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/testutils/unittest"
)

const fakeDepsContent = `# This is a fake DEPS file.

# Extraneous declarations shouldn't trip us up.
unknown_thing = 'ok'

vars = {
  'variable_repo': 'https://my-host/var-repo.git',
  'variable_revision': 'var-revision',
  'format_repo': 'https://my-host/format-repo.git',
  'format_revision': 'format-revision',
  'dict_revision': 'dict-revision',
}

deps = {
  'simple/dep': 'https://my-host/simple-repo.git@simple-revision',
  'variable/dep': Var('variable_repo') + '@' + Var('variable_revision'),
  'format/dep' : '{format_repo}@{format_revision}',
  'dict/dep': {
    'url': 'https://my-host/dict-repo.git@' + Var('dict_revision'),
    'condition': 'always!!!1',
  },
  'cipd/deps': {
    'packages': [
      {
        'package': 'package1',
        'version': 'version1',
      },
      {
        'package': 'package2/${{platform}}',
        'version': 'version2',
      },
    ],
    'dep_type': 'cipd',
  },
}
`

func TestParseDeps(t *testing.T) {
	unittest.SmallTest(t)

	// Verify that we parse the DEPS content successfully and get the
	// correct results for our toy example.
	deps, poss, err := parseDeps(fakeDepsContent)
	require.NoError(t, err)
	require.Equal(t, len(deps), len(poss))
	assertdeep.Equal(t, DepsEntries{
		"my-host/simple-repo": {
			Id:      "my-host/simple-repo",
			Version: "simple-revision",
			Path:    "simple/dep",
		},
		"my-host/var-repo": {
			Id:      "my-host/var-repo",
			Version: "var-revision",
			Path:    "variable/dep",
		},
		"my-host/format-repo": {
			Id:      "my-host/format-repo",
			Version: "format-revision",
			Path:    "format/dep",
		},
		"my-host/dict-repo": {
			Id:      "my-host/dict-repo",
			Version: "dict-revision",
			Path:    "dict/dep",
		},
		"package1": {
			Id:      "package1",
			Version: "version1",
			Path:    "cipd/deps",
		},
		"package2": {
			Id:      "package2",
			Version: "version2",
			Path:    "cipd/deps",
		},
	}, deps)

	// Positions should point to where the versions are defined.
	depsLines := strings.Split(fakeDepsContent, "\n")
	for idx, dep := range deps {
		pos := poss[idx]
		// Lineno starts at 1.
		lineIdx := pos.Lineno - 1
		str := depsLines[lineIdx][pos.ColOffset:]
		require.True(t, strings.Contains(str, dep.Version))
	}
}

func TestParseDepsRealWorld(t *testing.T) {
	// Manual test, since it loads data from real APIs.
	unittest.ManualTest(t)
	unittest.MediumTest(t)

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

func TestSetDep(t *testing.T) {
	unittest.SmallTest(t)

	before, beforePos, err := parseDeps(fakeDepsContent)
	require.NoError(t, err)
	beforeSplit := strings.Split(fakeDepsContent, "\n")

	// testSetDep runs SetDep and verifies that it performed the correct
	// modification.
	testSetDep := func(id, version string) {
		newDepsContent, err := SetDep(fakeDepsContent, id, version)
		require.NoError(t, err)
		after, afterPos, err := parseDeps(newDepsContent)
		require.NoError(t, err)
		var modifiedDepPos *ast.Pos
		for depId, dep := range after {
			expect := before[depId]
			if depId == NormalizeDep(id) {
				require.Equal(t, version, dep.Version)
				// Set the version back so we can use assertdeep.Equal.
				dep.Version = expect.Version
				modifiedDepPos = afterPos[depId]
			}
			assertdeep.Equal(t, expect, dep)
			assertdeep.Equal(t, beforePos[depId], afterPos[depId])
		}
		require.NotNil(t, modifiedDepPos, "Failed to find modified dep %q", id)
		modifiedLineIdx := modifiedDepPos.Lineno - 1 // Lineno starts at 1.

		// Ensure that we changed exactly one line.
		afterSplit := strings.Split(newDepsContent, "\n")
		require.Equal(t, len(beforeSplit), len(afterSplit))
		for idx, expectLine := range beforeSplit {
			actualLine := afterSplit[idx]
			if idx == modifiedLineIdx {
				require.NotEqual(t, expectLine, actualLine)
				require.True(t, strings.Contains(actualLine, version))
			} else {
				require.Equal(t, expectLine, actualLine)
			}
		}
	}

	// Run testSetDep for each dependency in fakeDepsContent.
	testSetDep("https://my-host/simple-repo.git", "newrev")
	testSetDep("my-host/simple-repo", "newrev") // Already normalized.
	testSetDep("https://my-host/var-repo.git", "newrev")
	testSetDep("https://my-host/dict-repo.git", "newrev")
	testSetDep("package1", "newrev")
	testSetDep("package2", "newrev")
}
