// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package filesystem

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/luci/luci-go/common/errors"
)

type luciPath string

func newLUCIPath(toks ...string) luciPath {
	return luciPath(path.Join(toks...))
}

func (l luciPath) explode() []string {
	return strings.Split(l.s(), "/")
}

func (l luciPath) toNative() nativePath {
	return nativePath(filepath.FromSlash(l.s()))
}

func (l luciPath) s() string {
	return string(l)
}

type configSet struct{ luciPath }

func newConfigSet(toks ...string) configSet {
	return configSet{newLUCIPath(toks...)}
}

func (c configSet) isProject() bool {
	return strings.Count(c.s(), "/") == 1 && c.hasPrefix("projects/")
}

func (c configSet) isProjectRef() bool {
	toks := c.explode()
	return len(toks) > 3 && toks[0] == "projects" && toks[2] == "refs"
}

func (c configSet) hasPrefix(prefix luciPath) bool {
	return strings.HasPrefix(c.s(), prefix.s())
}

func (c configSet) id() string {
	return strings.Split(c.s(), "/")[1]
}

func (c configSet) validate() error {
	if !c.hasPrefix("projects/") && !c.hasPrefix("services/") {
		return errors.Reason("configSet.validate: bad prefix %(cs)q").D("cs", c.s()).Err()
	}
	return nil
}

type nativePath string

func (n nativePath) explode() []string {
	return strings.Split(n.s(), string(filepath.Separator))
}

func (n nativePath) readlink() (nativePath, error) {
	ret, err := os.Readlink(n.s())
	if filepath.IsAbs(ret) {
		return nativePath(ret), err
	}
	return nativePath(filepath.Join(filepath.Dir(n.s()), ret)), err
}

func (n nativePath) rel(other nativePath) (nativePath, error) {
	ret, err := filepath.Rel(n.s(), other.s())
	return nativePath(ret), err
}

func (n nativePath) read() ([]byte, error) {
	return ioutil.ReadFile(n.s())
}

func (n nativePath) toLUCI() luciPath {
	return luciPath(filepath.ToSlash(n.s()))
}

func (n nativePath) s() string {
	return string(n)
}
