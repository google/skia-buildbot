// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package identity

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// Glob is glob like pattern that matches identity strings of some kind.
//
// It is a string of the form "kind:<pattern>" where 'kind' is one of Kind
// constants and 'pattern' is Unix-style wildcard pattern to apply to identity
// name.
//
// Supported syntax:
//  - '*' matches everything
//  - '?' matches any single character
//  - [seq] matches any character in seq
//  - [!seq] matches any character not in seq
//
// For a literal match, wrap the meta-characters in brackets. For example,
// '[?]' matches the character '?'.
type Glob string

// MakeGlob ensures 'glob' string looks like a valid identity glob and
// returns it as Glob value.
func MakeGlob(glob string) (Glob, error) {
	g := Glob(glob)
	if err := g.Validate(); err != nil {
		return "", err
	}
	return g, nil
}

// Validate checks that the identity glob string is well-formed.
func (g Glob) Validate() error {
	chunks := strings.SplitN(string(g), ":", 2)
	if len(chunks) != 2 {
		return fmt.Errorf("auth: bad identity glob string %q", g)
	}
	if knownKinds[Kind(chunks[0])] == nil {
		return fmt.Errorf("auth: bad identity glob kind %q", chunks[0])
	}
	if chunks[1] == "" {
		return fmt.Errorf("auth: identity glob can't be empty")
	}
	if _, err := cache.translate(chunks[1]); err != nil {
		return fmt.Errorf("auth: bad identity glob pattern %q - %s", chunks[1], err)
	}
	return nil
}

// Kind returns identity glob kind. If identity glob string is invalid returns
// Anonymous.
func (g Glob) Kind() Kind {
	chunks := strings.SplitN(string(g), ":", 2)
	if len(chunks) != 2 {
		return Anonymous
	}
	return Kind(chunks[0])
}

// Pattern returns a pattern part of the identity glob. If the identity glob
// string is invalid returns empty string.
func (g Glob) Pattern() string {
	chunks := strings.SplitN(string(g), ":", 2)
	if len(chunks) != 2 {
		return ""
	}
	return chunks[1]
}

// Match returns true if glob matches an identity. If identity string
// or identity glob string are invalid, returns false.
func (g Glob) Match(id Identity) bool {
	globChunks := strings.SplitN(string(g), ":", 2)
	if len(globChunks) != 2 || knownKinds[Kind(globChunks[0])] == nil {
		return false
	}
	globKind := globChunks[0]
	pattern := globChunks[1]

	idChunks := strings.SplitN(string(id), ":", 2)
	if len(idChunks) != 2 || knownKinds[Kind(idChunks[0])] == nil {
		return false
	}
	idKind := idChunks[0]
	name := idChunks[1]

	if idKind != globKind {
		return false
	}

	re, err := cache.translate(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

////

var cache patternCache

// patternCache implements caching for compiled pattern regexps, similar to how
// python does that. Uses extremely dumb algorithm that assumes there are less
// than 500 active patterns in the process. That's how python runtime does it
// too.
type patternCache struct {
	sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	re  *regexp.Regexp
	err error
}

// translate grabs converted regexp from cache or calls 'translate' to get it.
func (c *patternCache) translate(pat string) (*regexp.Regexp, error) {
	c.Lock()
	defer c.Unlock()
	if val, ok := c.cache[pat]; ok {
		return val.re, val.err
	}
	if c.cache == nil || len(c.cache) > 500 {
		c.cache = map[string]cacheEntry{}
	}
	re, err := regexp.Compile(translate(pat))
	c.cache[pat] = cacheEntry{re, err}
	return re, err
}

// translate converts glob pattern to a regular expression string. It is line
// by line port of python fnmatch.translate, since that's what is used on
// auth_service. Uglyness is inherited from the python code.
func translate(pat string) string {
	res := ""
	for i, n := 0, len(pat); i < n; {
		c := pat[i]
		i++
		switch c {
		case '*':
			res += ".*"
		case '?':
			res += "."
		case '[':
			j := i
			if j < n && pat[j] == '!' {
				j++
			}
			if j < n && pat[j] == ']' {
				j++
			}
			for j < n && pat[j] != ']' {
				j++
			}
			if j >= n {
				res += "\\["
			} else {
				stuff := strings.Replace(pat[i:j], "\\", "\\\\", -1)
				i = j + 1
				switch stuff[0] {
				case '!':
					stuff = "^" + stuff[1:]
				case '^':
					stuff = "\\" + stuff
				}
				res += "[" + stuff + "]"
			}
		default:
			res += regexp.QuoteMeta(string(c))
		}
	}
	return res + "\\z(?ms)"
}
