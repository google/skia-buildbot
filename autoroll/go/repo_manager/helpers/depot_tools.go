package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const parseDepsScript = `import json
import sys

# Read the DEPS file.
globals = {}
globals['Var'] = lambda key: globals.get('vars', {}).get(key)
execfile(sys.argv[1], globals)

def git_dep(url):
  split = url.split('@')
  if len(split) != 2:
    raise Exception('Invalid DEPS format: %s' % url)
  return {
    'git': {
      'repo': split[0],
      'revision': split[1],
    },
  }

# Organize into a saner format.
deps = {}
for k, v in globals['deps'].iteritems():
  if isinstance(v, str):
    deps[k] = git_dep(v)
  elif isinstance(v, dict):
    dep = None
    dep_type = v.get('dep_type', 'git')
    if dep_type == 'git':
      dep = git_dep(v['url'])
    elif dep_type == 'cipd':
      dep = {
        'cipd': {
          'packages': v['packages'],
        },
      }
    else:
      raise Exception('Unknown DEP type: %s', dep_type)
    if v.get('condition'):
      dep['condition'] = v['condition']
    deps[k] = dep
  else:
    raise Exception('Invalid DEPS format: %s' % v)
print json.dumps(deps, indent=2)`

// GetDEPSFile downloads and returns the path to the DEPS file, and a cleanup
// function to run when finished with it.
func GetDEPSFile(ctx context.Context, repo *gitiles.Repo, baseCommit string) (rv string, cleanup func(), rvErr error) {
	wd, err := ioutil.TempDir("", "")
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if rvErr != nil {
			util.RemoveAll(wd)
		}
	}()

	// Download the DEPS file from the parent repo.
	buf := bytes.NewBuffer([]byte{})
	if err := repo.ReadFileAtRef(ctx, "DEPS", baseCommit, buf); err != nil {
		return "", nil, err
	}

	depsFile := path.Join(wd, "DEPS")
	if err := ioutil.WriteFile(depsFile, buf.Bytes(), os.ModePerm); err != nil {
		return "", nil, err
	}
	return depsFile, func() { util.RemoveAll(wd) }, nil
}

func SetDep(ctx context.Context, gclient, depsFile, depPath, rev string) error {
	args := []string{"setdep", "-r", fmt.Sprintf("%s@%s", depPath, rev)}
	_, err := exec.RunCommand(ctx, &exec.Command{
		Dir:  path.Dir(depsFile),
		Env:  depot_tools.Env(filepath.Dir(gclient)),
		Name: gclient,
		Args: args,
	})
	return err
}

type jsonPackage struct {
	Package string `json:"package"`
	Version string `json:"version"`
}

type jsonCipdEntry struct {
	Packages []*jsonPackage `json:"packages"`
}

type jsonGitEntry struct {
	Repo     string `json:"repo"`
	Revision string `json:"revision'`
}

type jsonDepsEntry struct {
	// Exactly one of Cipd or Git should be defined.
	Cipd *jsonCipdEntry `json:"cipd"`
	Git  *jsonGitEntry  `json:'git"`

	Condition string `json:"condition"`
}

type DepsEntry struct {
	Id      string
	Version string
	Path    string
}

func RevInfo(ctx context.Context, depsFile string) (map[string]*DepsEntry, error) {
	// TODO(borenet): We shouldn't parse the DEPS file, but gclient doesn't
	// have a "dump all DEPS" command.
	out, err := exec.RunCwd(ctx, ".", "python", "-c", parseDepsScript, depsFile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var deps map[string]*jsonDepsEntry
	if err := json.NewDecoder(bytes.NewBuffer([]byte(out))).Decode(&deps); err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := map[string]*DepsEntry{}
	for path, dep := range deps {
		if dep.Git != nil {
			// TODO(borenet): Should we normalize the repo URL in
			// case different DEPS files use slightly different
			// formats, eg. absence of ".git"?
			rv[dep.Git.Repo] = &DepsEntry{
				Id:      dep.Git.Repo,
				Version: dep.Git.Revision,
				Path:    path,
			}
		} else if dep.Cipd != nil {
			for _, pkg := range dep.Cipd.Packages {
				rv[pkg.Package] = &DepsEntry{
					Id:      pkg.Package,
					Version: pkg.Version,
					Path:    path,
				}
			}
		} else {
			return nil, skerr.Fmt("No dep type for %q!", path)
		}
	}
	return rv, nil
}
