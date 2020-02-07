package isolate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestRead(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()

	type mkIsolateFn func(string, string)
	test := func(fn func(mkIsolateFn) (string, []string)) {
		tmp, cleanup := testutils.TempDir(t)
		defer cleanup()
		mk := func(path, content string) {
			fullPath := filepath.Join(tmp, path)
			dir := filepath.Dir(fullPath)
			if dir != "" {
				require.NoError(t, os.MkdirAll(dir, os.ModePerm))
			}
			testutils.WriteFile(t, fullPath, content)
		}
		startWith, expect := fn(mk)
		actual, err := Read(ctx, tmp, startWith)
		require.NoError(t, err)
		assertdeep.Equal(t, expect, actual)
	}

	// Empty isolate files.
	test(func(mk mkIsolateFn) (string, []string) {
		mk("infra/bots/start.isolate", `{}`)
		return "infra/bots/start.isolate", []string{}
	})

	test(func(mk mkIsolateFn) (string, []string) {
		mk("infra/bots/start.isolate", `{
  'variables': {
    'files': [],
  },
}`)
		return "infra/bots/start.isolate", []string{}
	})

	// Paths referenced in isolates are relative to the directory of the
	// isolate file itself.
	test(func(mk mkIsolateFn) (string, []string) {
		mk("infra/bots/start.isolate", `{
  'variables': {
    'files': ['../../f.txt'],
  },
}`)
		return "infra/bots/start.isolate", []string{"f.txt"}
	})

	test(func(mk mkIsolateFn) (string, []string) {
		mk("infra/bots/start.isolate", `{
  'variables': {
    'files': ['../../f.txt', '../somedir/', 'f2.txt'],
  },
}`)
		return "infra/bots/start.isolate", []string{
			"f.txt",
			"infra/bots/f2.txt",
			"infra/somedir", // os.path.normpath strips trailing "/"
		}
	})

	// Files outside of the given repo should not be included.
	test(func(mk mkIsolateFn) (string, []string) {
		mk("infra/bots/start.isolate", `{
  'variables': {
    'files': ['../../f.txt', '../../../.gclient'],
  },
}`)
		return "infra/bots/start.isolate", []string{"f.txt"}
	})

	// Isolate files may include other isolate files.
	test(func(mk mkIsolateFn) (string, []string) {
		mk("infra/bots/start.isolate", `{
  'includes': [
    '../../tools/include-only.isolate',
    'included.isolate',
  ],
  'variables': {
    'files': ['../../f.txt'],
  },
}`)
		mk("infra/bots/included.isolate", `{
  'variables': {
    'files': ['../file-from-include.txt'],
  },
}`)
		mk("tools/include-only.isolate", `{
  'includes': [
    'a/a.isolate',
    'b/b.isolate',
  ],
}`)
		mk("tools/a/a.isolate", `{
  'variables': {
    'files': ['a.txt'],
  },
}`)
		mk("tools/b/b.isolate", `{
  'variables': {
    'files': ['b.txt'],
  },
}`)
		return "infra/bots/start.isolate", []string{
			"f.txt",
			"infra/file-from-include.txt",
			"tools/a/a.txt",
			"tools/b/b.txt",
		}
	})
}
