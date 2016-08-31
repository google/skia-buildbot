// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package filesystem implements a file system backend for the config client.
//
// May be useful during local development.
//
// Layout
//
// A "Config Folder" has the following format:
//   - ./services/<servicename>/...
//   - ./projects/<projectname>.json
//   - ./projects/<projectname>/...
//   - ./projects/<projectname>/refs/<refname>/...
//
// Where `...` indicates any arbitrary path-to-a-file, and <brackets> indicate
// a single non-slash-containing filesystem path token. "services", "projects",
// ".json", and "refs" and slashes are all literal text.
//
// This package allows two modes of operation
//
// Symlink Mode
//
// This mode allows you to simulate the evolution of multiple configuration
// versions during the duration of your test. Lay out your entire directory
// structure like:
//
//   - ./current -> ./v1
//   - ./v1/config_folder/...
//   - ./v2/config_folder/...
//
// During the execution of your app, you can change ./current from v1 to v2 (or
// any other version), and that will be reflected in the config client's
// Revision field. That way you may "simulate" atomic changes in the
// configuration. You would pass the path to `current` as the basePath in the
// constructor of New.
//
// Sloppy Version Mode
//
// The folder will be scanned each time a config file is accessed, and the
// Revision will be derived based on the current content of all config files.
// Some inconsistencies are possible if configs change during the directory
// rescan (thus "sloppiness" of this mode). This is good if you just want to
// be able to easily modify configs manually during the development without
// restarting the server or messing with symlinks.
//
// Quirks
//
// This implementation is quite dumb, and will scan the entire directory each
// time configs are accessed, caching the whole thing in memory (content, hashes
// and metadata) and never cleaning it up. This means that if you keep editing
// the files, more and more stuff will accumulate in memory.
package filesystem

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/errors"
)

// ProjectConfiguration is the struct that will be used to read the
// `projectname.json` config file, if any is specified for a given project.
type ProjectConfiguration struct {
	Name string
	URL  string
}

type lookupKey struct {
	revision  string
	configSet configSet
	path      luciPath
}

type filesystemImpl struct {
	sync.RWMutex
	scannedConfigs

	basePath nativePath
	islink   bool

	contentRevisionsScanned stringset.Set
}

type scannedConfigs struct {
	contentHashMap    map[string]string
	contentRevPathMap map[lookupKey]*config.Config
	contentRevProject map[lookupKey]*config.Project
}

func newScannedConfigs() scannedConfigs {
	return scannedConfigs{
		contentHashMap:    map[string]string{},
		contentRevPathMap: map[lookupKey]*config.Config{},
		contentRevProject: map[lookupKey]*config.Project{},
	}
}

// setRevision updates 'revision' fields of all objects owned by scannedConfigs.
func (c *scannedConfigs) setRevision(revision string) {
	newRevPathMap := make(map[lookupKey]*config.Config, len(c.contentRevPathMap))
	for k, v := range c.contentRevPathMap {
		k.revision = revision
		v.Revision = revision
		newRevPathMap[k] = v
	}
	c.contentRevPathMap = newRevPathMap

	newRevProject := make(map[lookupKey]*config.Project, len(c.contentRevProject))
	for k, v := range c.contentRevProject {
		k.revision = revision
		newRevProject[k] = v
	}
	c.contentRevProject = newRevProject
}

// deriveRevision generates a revision string from data in contentHashMap.
func deriveRevision(c *scannedConfigs) string {
	keys := make([]string, 0, len(c.contentHashMap))
	for k := range c.contentHashMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	hsh := sha1.New()
	for _, k := range keys {
		fmt.Fprintf(hsh, "%s\n%s\n", k, c.contentHashMap[k])
	}
	digest := hsh.Sum(nil)
	return hex.EncodeToString(digest[:])
}

// New returns an implementation of the config service which reads configuration
// from the local filesystem. `basePath` may be one of two things:
//   * A folder containing the following:
//     ./services/servicename/...               # service configuations
//     ./projects/projectname.json              # project information configuation
//     ./projects/projectname/...               # project configuations
//     ./projects/projectname/refs/refname/...  # project ref configuations
//   * A symlink to a folder as organized above:
//     -> /path/to/revision/folder
//
// If a symlink is used, all Revision fields will be the 'revision' portion of
// that path. If a non-symlink path is isued, the Revision fields will be
// derived based on the contents of the files in the directory.
//
// Any unrecognized paths will be ignored. If basePath is not a link-to-folder,
// and not a folder, this will panic.
//
// Every read access will scan each revision exactly once. If you want to make
// changes, rename the folder and re-link it.
func New(basePath string) (config.Interface, error) {
	basePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, err
	}

	inf, err := os.Lstat(basePath)
	if err != nil {
		return nil, err
	}

	ret := &filesystemImpl{
		basePath:                nativePath(basePath),
		islink:                  (inf.Mode() & os.ModeSymlink) != 0,
		scannedConfigs:          newScannedConfigs(),
		contentRevisionsScanned: stringset.New(1),
	}

	if ret.islink {
		inf, err := os.Stat(basePath)
		if err != nil {
			return nil, err
		}
		if !inf.IsDir() {
			return nil, (errors.Reason("filesystem.New(%(basePath)q): does not link to a directory").
				D("basePath", basePath).Err())
		}
		if len(ret.basePath.explode()) < 1 {
			return nil, (errors.Reason("filesystem.New(%(basePath)q): not enough tokens in path").
				D("basePath", basePath).Err())
		}
	} else if !inf.IsDir() {
		return nil, (errors.Reason("filesystem.New(%(basePath)q): not a directory").
			D("basePath", basePath).Err())
	}
	return ret, nil
}

func (fs *filesystemImpl) resolveBasePath() (realPath nativePath, revision string, err error) {
	if fs.islink {
		realPath, err = fs.basePath.readlink()
		if err != nil && err.(*os.PathError).Err != os.ErrInvalid {
			return
		}
		toks := realPath.explode()
		revision = toks[len(toks)-1]
		return
	}
	return fs.basePath, "", nil
}

func parsePath(rel nativePath) (configSet configSet, path luciPath, ok bool) {
	toks := rel.explode()

	const jsonExt = ".json"

	if toks[0] == "services" {
		configSet = newConfigSet(toks[:2]...)
		path = newLUCIPath(toks[2:]...)
		ok = true
	} else if toks[0] == "projects" {
		ok = true
		if len(toks) > 2 && toks[2] == "refs" { // projects/p/refs/r/...
			if len(toks) > 4 {
				configSet = newConfigSet(toks[:4]...)
				path = newLUCIPath(toks[4:]...)
			} else {
				// otherwise it's invalid /projects/p/refs or /projects/p/refs/somefile
				ok = false
			}
		} else if len(toks) == 2 && strings.HasSuffix(toks[1], jsonExt) {
			configSet = newConfigSet(toks[0], toks[1][:len(toks[1])-len(jsonExt)])
		} else {
			configSet = newConfigSet(toks[:2]...)
			path = newLUCIPath(toks[2:]...)
		}
	}
	return
}

func scanDirectory(realPath nativePath) (*scannedConfigs, error) {
	ret := newScannedConfigs()

	err := filepath.Walk(realPath.s(), func(rawPath string, info os.FileInfo, err error) error {
		path := nativePath(rawPath)

		if err != nil {
			return err
		}

		if !info.IsDir() {
			rel, err := realPath.rel(path)
			if err != nil {
				return err
			}

			configSet, cfgPath, ok := parsePath(rel)
			if !ok {
				return nil
			}
			lk := lookupKey{"", configSet, cfgPath}

			data, err := path.read()
			if err != nil {
				return err
			}

			if cfgPath == "" { // this is the project configuration file
				proj := &ProjectConfiguration{}
				if err := json.Unmarshal(data, proj); err != nil {
					return err
				}
				toks := configSet.explode()
				parsedURL, err := url.ParseRequestURI(proj.URL)
				if err != nil {
					return err
				}
				ret.contentRevProject[lk] = &config.Project{
					ID:       toks[1],
					Name:     proj.Name,
					RepoType: "FILESYSTEM",
					RepoURL:  parsedURL,
				}
				return nil
			}

			content := string(data)

			hsh := sha1.Sum(data)
			hexHsh := "v1:" + hex.EncodeToString(hsh[:])

			ret.contentHashMap[hexHsh] = content

			ret.contentRevPathMap[lk] = &config.Config{
				ConfigSet: configSet.s(), Path: cfgPath.s(),
				Content:     content,
				ContentHash: hexHsh,
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	for lk := range ret.contentRevPathMap {
		cs := lk.configSet
		if cs.isProject() {
			pk := lookupKey{"", cs, ""}
			if ret.contentRevProject[pk] == nil {
				id := cs.id()
				ret.contentRevProject[pk] = &config.Project{
					ID:       id,
					Name:     id,
					RepoType: "FILESYSTEM",
				}
			}
		}
	}

	return &ret, nil
}

func (fs *filesystemImpl) scanHeadRevision() (string, error) {
	realPath, revision, err := fs.resolveBasePath()
	if err != nil {
		return "", err
	}

	// Using symlinks? The revision is derived from the symlink target name,
	// do not rescan it all the time.
	if revision != "" {
		if err := fs.scanSymlinkedRevision(realPath, revision); err != nil {
			return "", err
		}
		return revision, nil
	}

	// If using regular directory, rescan it to find if anything changed.
	return fs.scanCurrentRevision(realPath)
}

func (fs *filesystemImpl) scanSymlinkedRevision(realPath nativePath, revision string) error {
	fs.RLock()
	done := fs.contentRevisionsScanned.Has(revision)
	fs.RUnlock()
	if done {
		return nil
	}

	fs.Lock()
	defer fs.Unlock()

	scanned, err := scanDirectory(realPath)
	if err != nil {
		return err
	}
	fs.slurpScannedConfigs(revision, scanned)
	return nil
}

func (fs *filesystemImpl) scanCurrentRevision(realPath nativePath) (string, error) {
	// Forbid parallel scans to avoid hitting the disk too hard.
	//
	// TODO(vadimsh): Can use some sort of rate limiting instead if this code is
	// ever used in production.
	fs.Lock()
	defer fs.Unlock()

	scanned, err := scanDirectory(realPath)
	if err != nil {
		return "", err
	}

	revision := deriveRevision(scanned)
	if fs.contentRevisionsScanned.Has(revision) {
		return revision, nil // no changes to configs
	}
	fs.slurpScannedConfigs(revision, scanned)
	return revision, nil
}

func (fs *filesystemImpl) slurpScannedConfigs(revision string, scanned *scannedConfigs) {
	scanned.setRevision(revision)
	for k, v := range scanned.contentHashMap {
		fs.contentHashMap[k] = v
	}
	for k, v := range scanned.contentRevPathMap {
		fs.contentRevPathMap[k] = v
	}
	for k, v := range scanned.contentRevProject {
		fs.contentRevProject[k] = v
	}
	fs.contentRevisionsScanned.Add(revision)
}

func (fs *filesystemImpl) ServiceURL(ctx context.Context) url.URL {
	return url.URL{
		Scheme: "file",
		Path:   fs.basePath.s(),
	}
}

func (fs *filesystemImpl) GetConfig(ctx context.Context, cfgSet, cfgPath string, hashOnly bool) (*config.Config, error) {
	configSet := configSet{luciPath(cfgSet)}
	path := luciPath(cfgPath)

	if err := configSet.validate(); err != nil {
		return nil, err
	}

	revision, err := fs.scanHeadRevision()
	if err != nil {
		return nil, err
	}

	lk := lookupKey{revision, configSet, path}

	fs.RLock()
	ret, ok := fs.contentRevPathMap[lk]
	fs.RUnlock()
	if ok {
		return ret, nil
	}
	return nil, config.ErrNoConfig
}

func (fs *filesystemImpl) GetConfigByHash(ctx context.Context, contentHash string) (string, error) {
	if _, err := fs.scanHeadRevision(); err != nil {
		return "", err
	}

	fs.RLock()
	content, ok := fs.contentHashMap[contentHash]
	fs.RUnlock()
	if ok {
		return content, nil
	}
	return "", config.ErrNoConfig
}

func (fs *filesystemImpl) GetConfigSetLocation(ctx context.Context, cfgSet string) (*url.URL, error) {
	configSet := configSet{luciPath(cfgSet)}

	if err := configSet.validate(); err != nil {
		return nil, err
	}
	realPath, _, err := fs.resolveBasePath()
	if err != nil {
		return nil, err
	}
	return &url.URL{
		Scheme: "file",
		Path:   realPath.toLUCI().s() + "/" + configSet.s(),
	}, nil
}

func (fs *filesystemImpl) iterContentRevPath(fn func(lk lookupKey, cfg *config.Config)) error {
	revision, err := fs.scanHeadRevision()
	if err != nil {
		return err
	}

	fs.RLock()
	defer fs.RUnlock()
	for lk, cfg := range fs.contentRevPathMap {
		if lk.revision == revision {
			fn(lk, cfg)
		}
	}
	return nil
}

func (fs *filesystemImpl) GetProjectConfigs(ctx context.Context, cfgPath string, hashesOnly bool) ([]config.Config, error) {
	path := luciPath(cfgPath)

	ret := make(configList, 0, 10)
	err := fs.iterContentRevPath(func(lk lookupKey, cfg *config.Config) {
		if lk.path != path {
			return
		}
		if lk.configSet.isProject() {
			c := *cfg
			if hashesOnly {
				c.Content = ""
			}
			ret = append(ret, c)
		}
	})
	sort.Sort(ret)
	return ret, err
}

func (fs *filesystemImpl) GetProjects(ctx context.Context) ([]config.Project, error) {
	revision, err := fs.scanHeadRevision()
	if err != nil {
		return nil, err
	}

	fs.RLock()
	ret := make(projList, 0, len(fs.contentRevProject))
	for lk, proj := range fs.contentRevProject {
		if lk.revision == revision {
			ret = append(ret, *proj)
		}
	}
	fs.RUnlock()
	sort.Sort(ret)
	return ret, nil
}

func (fs *filesystemImpl) GetRefConfigs(ctx context.Context, cfgPath string, hashesOnly bool) ([]config.Config, error) {
	path := luciPath(cfgPath)

	ret := make(configList, 0, 10)
	err := fs.iterContentRevPath(func(lk lookupKey, cfg *config.Config) {
		if lk.path != path {
			return
		}
		if lk.configSet.isProjectRef() {
			c := *cfg
			if hashesOnly {
				c.Content = ""
			}
			ret = append(ret, c)
		}
	})
	sort.Sort(ret)
	return ret, err
}

func (fs *filesystemImpl) GetRefs(ctx context.Context, projectID string) ([]string, error) {
	pfx := luciPath("projects/" + projectID + "/refs")
	ret := stringset.New(0)
	err := fs.iterContentRevPath(func(lk lookupKey, cfg *config.Config) {
		if lk.configSet.hasPrefix(pfx) {
			ret.Add(newConfigSet(lk.configSet.explode()[2:]...).s())
		}
	})
	retSlc := ret.ToSlice()
	sort.Strings(retSlc)
	return retSlc, err
}
