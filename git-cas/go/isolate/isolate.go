package isolate

import (
	"context"
	"strings"

	"go.skia.org/infra/go/exec"
)

/*
	Package isolate reads isolate files so that they can be used with git-cas.
*/

const script = `
import ast
import os
import sys

def load_isolate(root, isolate):
  base = os.path.dirname(os.path.relpath(isolate, root))
  with open(isolate) as f:
    content = ast.literal_eval(f.read())
  files = set()
  for f in content.get('variables', {}).get('files', []):
    files.add(os.path.normpath(os.path.join(base, f)))
  for inc in content.get('includes', []):
    for f in load_isolate(root, os.path.join(base, inc)):
      files.add(f)
  return sorted(files)

print('\n'.join(load_isolate(sys.argv[1], sys.argv[2])))
`

// Read the given isolate file from the given repo root and return the list of
// paths it references.
func Read(ctx context.Context, root, isolate string) ([]string, error) {
	out, err := exec.RunCwd(ctx, root, "python", "-c", script, root, isolate)
	if err != nil {
		return nil, err
	}
	items := strings.Split(out, "\n")
	rv := make([]string, 0, len(items))
	for _, item := range items {
		if item != "" && !strings.HasPrefix(item, "..") {
			rv = append(rv, item)
		}
	}
	return rv, nil
}
