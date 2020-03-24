package deps_parser

import (
	"bytes"
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
  'variable_repo': 'var-repo.git',
  'variable_revision': 'var-revision',
  'dict_revision': 'dict-revision',
}

deps = {
  'simple/dep': 'simple-repo.git@simple-revision',
  'variable/dep': Var('variable_repo') + '@' + Var('variable_revision'),
  'dict/dep': {
    'url': 'dict-repo.git@' + Var('dict_revision'),
    'condition': 'always!!!1',
  },
  'cipd/deps': {
    'packages': [
      {
        'package': 'package1',
        'version': 'version1',
      },
      {
        'package': 'package2',
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
	assertdeep.Equal(t, map[string]*DepsEntry{
		"simple-repo.git": {
			Id:      "simple-repo.git",
			Version: "simple-revision",
			Path:    "simple/dep",
		},
		"var-repo.git": {
			Id:      "var-repo.git",
			Version: "var-revision",
			Path:    "variable/dep",
		},
		"dict-repo.git": {
			Id:      "dict-repo.git",
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
		var buf bytes.Buffer
		require.NoError(t, gitiles.NewRepo(repo, nil).ReadFileAtRef(ctx, "DEPS", rev, &buf))
		actual, poss, err := parseDeps(buf.String())
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
		"https://skia.googlesource.com/skia.git": {
			// Skia is a simple (non-dict) dep with a vars lookup.
			DepsEntry: &DepsEntry{
				Id:      "https://skia.googlesource.com/skia.git",
				Version: "85755f46a8810b1863493a81887f1dc17c2e49e1",
				Path:    "src/third_party/skia",
			},
			Pos: &ast.Pos{
				Lineno:    178,
				ColOffset: 19,
			},
		},
		"infra/tools/luci/swarming/${{platform}}": {
			// Swarming client is a CIPD dep with a vars lookup.
			DepsEntry: &DepsEntry{
				Id:      "infra/tools/luci/swarming/${{platform}}",
				Version: "git_revision:de73cf6c4bde86f0a9c8d54151b69b0154a398f1",
				Path:    "src/tools/luci-go",
			},
			Pos: &ast.Pos{
				Lineno:    159,
				ColOffset: 13,
			},
		},
		"https://android.googlesource.com/platform/external/protobuf.git": {
			// This is a dict entry whose version is defined in place.
			DepsEntry: &DepsEntry{
				Id:      "https://android.googlesource.com/platform/external/protobuf.git",
				Version: "7fca48d8ce97f7ba3ab8eea5c472f1ad3711762f",
				Path:    "src/third_party/android_protobuf/src",
			},
			Pos: &ast.Pos{
				Lineno:    640,
				ColOffset: 76,
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
			if depId == id {
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
	testSetDep("simple-repo.git", "newrev")
	testSetDep("var-repo.git", "newrev")
	testSetDep("dict-repo.git", "newrev")
	testSetDep("package1", "newrev")
	testSetDep("package2", "newrev")
}
