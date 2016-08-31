// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cache

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/luci/luci-go/common/isolated"
	"github.com/maruel/ut"
)

func testCache(t *testing.T, c Cache) isolated.HexDigests {
	// c's policies must have MaxItems == 2 and MaxSize == 1024.
	td, err := ioutil.TempDir("", "cache")
	ut.AssertEqual(t, nil, err)
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			t.Fail()
		}
	}()

	fakeDigest := isolated.HexDigest("0123456789012345678901234567890123456789")
	badDigest := isolated.HexDigest("012345678901234567890123456789012345678")
	emptyContent := []byte{}
	emptyDigest := isolated.HashBytes(emptyContent)
	file1Content := []byte("foo")
	file1Digest := isolated.HashBytes(file1Content)
	file2Content := []byte("foo bar")
	file2Digest := isolated.HashBytes(file2Content)
	largeContent := bytes.Repeat([]byte("A"), 1023)
	largeDigest := isolated.HashBytes(largeContent)
	tooLargeContent := bytes.Repeat([]byte("A"), 1025)
	tooLargeDigest := isolated.HashBytes(tooLargeContent)

	ut.AssertEqual(t, isolated.HexDigests{}, c.Keys())

	ut.AssertEqual(t, false, c.Touch(fakeDigest))
	ut.AssertEqual(t, false, c.Touch(badDigest))

	c.Evict(fakeDigest)
	c.Evict(badDigest)

	r, err := c.Read(fakeDigest)
	ut.AssertEqual(t, nil, r)
	ut.AssertEqual(t, true, err != nil)
	r, err = c.Read(badDigest)
	ut.AssertEqual(t, nil, r)
	ut.AssertEqual(t, true, err != nil)

	// It's too large to fit in the cache.
	ut.AssertEqual(t, true, nil != c.Add(tooLargeDigest, bytes.NewBuffer(tooLargeContent)))

	// It gets discarded because it's too large.
	ut.AssertEqual(t, nil, c.Add(largeDigest, bytes.NewBuffer(largeContent)))
	ut.AssertEqual(t, nil, c.Add(emptyDigest, bytes.NewBuffer(emptyContent)))
	ut.AssertEqual(t, nil, c.Add(emptyDigest, bytes.NewBuffer(emptyContent)))
	ut.AssertEqual(t, isolated.HexDigests{emptyDigest, largeDigest}, c.Keys())
	c.Evict(emptyDigest)
	ut.AssertEqual(t, isolated.HexDigests{largeDigest}, c.Keys())
	ut.AssertEqual(t, nil, c.Add(emptyDigest, bytes.NewBuffer(emptyContent)))

	ut.AssertEqual(t, nil, c.Add(file1Digest, bytes.NewBuffer(file1Content)))
	ut.AssertEqual(t, true, c.Touch(emptyDigest))
	ut.AssertEqual(t, nil, c.Add(file2Digest, bytes.NewBuffer(file2Content)))

	r, err = c.Read(file1Digest)
	ut.AssertEqual(t, nil, r)
	ut.AssertEqual(t, true, nil != err)
	r, err = c.Read(file2Digest)
	ut.AssertEqual(t, nil, err)
	actual, err := ioutil.ReadAll(r)
	ut.AssertEqual(t, nil, r.Close())
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, file2Content, actual)

	expected := isolated.HexDigests{file2Digest, emptyDigest}
	ut.AssertEqual(t, expected, c.Keys())

	dest := filepath.Join(td, "foo")
	ut.AssertEqual(t, true, nil != c.Hardlink(fakeDigest, dest, os.FileMode(0600)))
	ut.AssertEqual(t, true, nil != c.Hardlink(badDigest, dest, os.FileMode(0600)))
	ut.AssertEqual(t, nil, c.Hardlink(file2Digest, dest, os.FileMode(0600)))
	// See comment about the fact that it may or may not work.
	_ = c.Hardlink(file2Digest, dest, os.FileMode(0600))
	actual, err = ioutil.ReadFile(dest)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, file2Content, actual)

	ut.AssertEqual(t, nil, c.Close())
	return expected
}

func TestNewMemory(t *testing.T) {
	testCache(t, NewMemory(Policies{MaxSize: 1024, MaxItems: 2}))
}

func TestNewDisk(t *testing.T) {
	td, err := ioutil.TempDir("", "cache")
	ut.AssertEqual(t, nil, err)
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			t.Error(err)
		}
	}()
	pol := Policies{MaxSize: 1024, MaxItems: 2}
	c, err := NewDisk(pol, td)
	ut.AssertEqual(t, nil, err)
	expected := testCache(t, c)

	c, err = NewDisk(pol, td)
	ut.AssertEqual(t, nil, err)
	ut.AssertEqual(t, expected, c.Keys())
	ut.AssertEqual(t, nil, c.Close())

	c, err = NewDisk(pol, "non absolute path")
	ut.AssertEqual(t, nil, c)
	ut.AssertEqual(t, true, nil != err)
}
