// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package isolate

import (
	"errors"
	"log"
	"path"
	"strings"

	// TODO(tandrii): Remove this testing lib from non-test file.
	"github.com/maruel/ut"
)

// TODO(tandrii): Remove this hacky stuff.
// hacky stuff for faster debugging.
func assertNoError(err error) {
	if err == nil {
		return
	}
	log.Panicf(ut.Decorate("assertion failed due to error %s."), err)
}

func assert(condition bool, info ...interface{}) {
	if condition {
		return
	}
	if len(info) == 0 {
		log.Panic(ut.Decorate("assertion failed"))
	} else if format, ok := info[0].(string); ok {
		log.Panicf(ut.Decorate("assertion failed: "+format), info[1:]...)
	}
}

// uniqueMergeSortedStrings merges two sorted sets of string (as slices) and removes duplicates.
func uniqueMergeSortedStrings(ls, rs []string) []string {
	varSet := make([]string, len(ls)+len(rs))
	for i := 0; ; i++ {
		if len(ls) == 0 {
			rs, ls = ls, rs
		}
		if len(rs) == 0 {
			i += copy(varSet[i:], ls)
			return varSet[:i]
		}
		assert(i < len(varSet))
		assert(len(rs) > 0 && len(ls) > 0)
		if ls[0] > rs[0] {
			ls, rs = rs, ls
		}
		if ls[0] < rs[0] {
			varSet[i] = ls[0]
			ls = ls[1:]
		} else {
			varSet[i] = ls[0]
			ls, rs = ls[1:], rs[1:]
		}
	}
}

// posixRel returns a relative path that is lexically equivalent to targpath when
// joined to basepath with an intervening separator.
//
// That is, Join(basepath, Rel(basepath, targpath)) is equivalent to targpath itself.
// On success, the returned path will always be relative to basepath,
// even if basepath and targpath share no elements.
// An error is returned if targpath can't be made relative to basepath or if
// knowing the current working directory would be necessary to compute it.
//
// Copy-pasted & slightly edited from Go's lib path/filepath/path.go .
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
func posixRel(basepath, targpath string) (string, error) {
	base := path.Clean(basepath)
	targ := path.Clean(targpath)
	if targ == base {
		return ".", nil
	}
	if base == "." {
		base = ""
	}
	if path.IsAbs(base) != path.IsAbs(targ) {
		return "", errors.New("Rel: can't make " + targ + " relative to " + base)
	}
	// Position base[b0:bi] and targ[t0:ti] at the first differing elements.
	bl := len(base)
	tl := len(targ)
	var b0, bi, t0, ti int
	for {
		for bi < bl && base[bi] != '/' {
			bi++
		}
		for ti < tl && targ[ti] != '/' {
			ti++
		}
		if targ[t0:ti] != base[b0:bi] {
			break
		}
		if bi < bl {
			bi++
		}
		if ti < tl {
			ti++
		}
		b0 = bi
		t0 = ti
	}
	if base[b0:bi] == ".." {
		return "", errors.New("Rel: can't make " + targ + " relative to " + base)
	}
	if b0 != bl {
		// Base elements left. Must go up before going down.
		seps := strings.Count(base[b0:bl], string('/'))
		size := 2 + seps*3
		if tl != t0 {
			size += 1 + tl - t0
		}
		buf := make([]byte, size)
		n := copy(buf, "..")
		for i := 0; i < seps; i++ {
			buf[n] = '/'
			copy(buf[n+1:], "..")
			n += 3
		}
		if t0 != tl {
			buf[n] = '/'
			copy(buf[n+1:], targ[t0:])
		}
		return string(buf), nil
	}
	return targ[t0:], nil
}

func newInt(v int) *int {
	o := new(int)
	*o = v
	return o
}

func newInt64(v int64) *int64 {
	o := new(int64)
	*o = v
	return o
}

func newString(v string) *string {
	o := new(string)
	*o = v
	return o
}

func stringToRunes(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		out = append(out, r)
	}
	return out
}
