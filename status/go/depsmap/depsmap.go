package depsmap

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/util"
)

const (
	cacheFile = "depsmap.gob"

	parseDepsFile = `import json
import os
import sys

with open(sys.argv[1]) as f:
  content = f.read()

loc = {}

def getvar(name):
  return loc['vars'][name]

loc['Var'] = getvar

exec(content, {}, loc)
print os.listdir(os.path.dirname(sys.argv[2]))
with open(sys.argv[2], 'w') as f:
  f.write(json.dumps(loc.get('deps', {})))
`
)

// DepsMap is a struct which maps parent repo commit hashes to child commit
// hashes as specified in the DEPS file.
type DepsMap struct {
	deps    map[string]map[string]string
	mtx     sync.RWMutex
	r       *repograph.Graph
	workdir string
}

// gobDepsMap is a utility struct used for serializing a DepsMap using gob.
type gobDepsMap struct {
	Deps map[string]map[string]string
}

// New returns a new DepsMap instance. The caller is responsible for managing
// the given git.Repo instance, including updating it.
func New(repoUrl, workdir string) (*DepsMap, error) {
	repo, err := repograph.NewGraph(repoUrl, workdir)
	if err != nil {
		return nil, err
	}
	rv := &DepsMap{
		deps:    map[string]map[string]string{},
		r:       repo,
		workdir: workdir,
	}
	if err := rv.read(); err != nil {
		return nil, err
	}
	return rv, rv.Update()
}

// read reads the cached DepsMap from a file.
func (d *DepsMap) read() error {
	cached := path.Join(d.workdir, cacheFile)
	f, err := os.Open(cached)
	if err == nil {
		defer util.Close(f)
		var m gobDepsMap
		if err := gob.NewDecoder(f).Decode(&m); err != nil {
			return err
		}
		d.deps = m.Deps
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Failed to read cache file: %s", err)
	}
	return nil
}

// write writes the DepsMap to a cache file.
func (d *DepsMap) write() error {
	cached := path.Join(d.workdir, cacheFile)
	f, err := os.Create(cached)
	if err != nil {
		return err
	}
	defer util.Close(f)
	m := gobDepsMap{
		Deps: d.deps,
	}
	return gob.NewEncoder(f).Encode(m)
}

// Update updates the DepsMap.
func (d *DepsMap) Update() error {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	// Update the repo.
	if err := d.r.Update(); err != nil {
		return err
	}

	// Load DEPS for all new commits.
	if err := d.r.RecurseAllBranches(func(c *repograph.Commit) (bool, error) {
		// Stop recursing if we've already read DEPS for this commit.
		if _, ok := d.deps[c.Hash]; ok {
			return false, nil
		}

		// Read DEPS.
		deps, err := d.readDepsFile(c.Hash)
		if err != nil {
			return false, err
		}
		d.deps[c.Hash] = deps
		return true, nil
	}); err != nil {
		return err
	}

	return d.write()
}

// Lookup returns the hash of the child DEP at the given parent commit.
func (d *DepsMap) Lookup(parentHash, childRepo string) (string, error) {
	d.mtx.RLock()
	defer d.mtx.RUnlock()

	// Get the DEPS at the given parentHash.
	deps, ok := d.deps[parentHash]
	if !ok {
		return "", fmt.Errorf("DepsMap has no DEPS at %q", parentHash)
	}

	// Look up the DEP for childRepo.
	if h, ok := deps[childRepo]; ok {
		return h, nil
	}
	return "", fmt.Errorf("No such DEP %q at parent commit %q", childRepo, parentHash)
}

// readDepsFile executes the DEPS file at the given revision of the parent repo,
// then returns a map[string]string whose keys are child repo URLs and values
// are child commit hashes.
func (d *DepsMap) readDepsFile(parentHash string) (map[string]string, error) {
	// Ensure that the Python script exists.
	script := path.Join(d.workdir, "parse_deps.py")
	if err := ioutil.WriteFile(script, []byte(parseDepsFile), os.ModePerm); err != nil {
		return nil, err
	}

	// Read the DEPS file at the given commit.
	contents, err := d.r.Repo().Git("show", fmt.Sprintf("%s:DEPS", parentHash))
	if err != nil {
		// No DEPS file, no DEPS, this is okay.
		if strings.Contains(err.Error(), "Path 'DEPS' does not exist in") {
			return map[string]string{}, nil
		}
		return nil, err
	}
	depsContents := path.Join(d.workdir, "DEPS")
	if err := ioutil.WriteFile(depsContents, []byte(contents), os.ModePerm); err != nil {
		return nil, err
	}

	// Parse the DEPS file.
	outFile := path.Join(d.workdir, "out.json")
	if _, err := exec.RunCwd(d.workdir, "python", script, depsContents, outFile); err != nil {
		return nil, err
	}

	// Parse the JSON.
	out, err := ioutil.ReadFile(outFile)
	if err != nil {
		return nil, err
	}

	deps := map[string]string{}
	if err := json.Unmarshal(out, &deps); err != nil {
		return nil, err
	}
	rv := make(map[string]string, len(deps))
	for _, dep := range deps {
		split := strings.SplitN(dep, "@", 2)
		if len(split) == 1 {
			rv[dep] = "HEAD"
		} else if len(split) == 2 {
			rv[split[0]] = split[1]
		} else {
			return nil, fmt.Errorf("Unable to parse DEPS file; invalid DEP: %s", dep)
		}
	}
	return rv, nil
}
