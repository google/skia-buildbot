// Copyright 2014 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package local

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
)

// NewTestFile returns File implementation (Symlink == false) backed by a fake
// in-memory data. It is useful in unit tests.
func NewTestFile(name string, data string, executable bool) File {
	return &testFile{
		name:       name,
		data:       data,
		executable: executable,
	}
}

// NewTestSymlink returns File implementation (Symlink == true) backed by a fake
// in-memory data. It is useful in unit tests.
func NewTestSymlink(name string, target string) File {
	return &testFile{
		name:          name,
		symlinkTarget: target,
	}
}

type testFile struct {
	name          string
	data          string
	executable    bool
	symlinkTarget string
}

func (f *testFile) Name() string     { return f.name }
func (f *testFile) Size() uint64     { return uint64(len(f.data)) }
func (f *testFile) Executable() bool { return f.executable }
func (f *testFile) Symlink() bool    { return f.symlinkTarget != "" }

func (f *testFile) SymlinkTarget() (string, error) {
	if f.symlinkTarget == "" {
		return "", fmt.Errorf("not a symlink: %s", f.Name())
	}
	return f.symlinkTarget, nil
}

func (f *testFile) Open() (io.ReadCloser, error) {
	if f.Symlink() {
		return nil, fmt.Errorf("can't open symlink: %s", f.Name())
	}
	r := bytes.NewReader([]byte(f.data))
	return ioutil.NopCloser(r), nil
}
