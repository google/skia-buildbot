// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gs

import (
	"strings"
)

// Path is a Google Storage path. A full path consists of a Google storage
// bucket and a series of path components.
//
// An example of a Path is:
//	gs://test-bucket/path/to/thing.txt
type Path string

// MakePath constructs a Google Storage path from optional bucket and filename
// components.
//
// Trailing forward slashes will be removed from the bucket name, if present.
func MakePath(bucket, filename string) Path {
	var carr [2]string

	comps := carr[:0]
	if b := stripTrailingSlashes(bucket); b != "" {
		comps = append(comps, "gs://"+b)
	}
	if filename != "" {
		comps = append(comps, filename)
	}
	return Path(strings.Join(comps, "/"))
}

// Bucket returns the Google Storage bucket component of the Path. If there is
// no bucket, an empty string will be returned.
func (p Path) Bucket() string {
	b, _ := p.Split()
	return b
}

// Filename returns the filename component of the Path. If there is no filename
// component, an empty string will be returned.
//
// Leading and trailing slashes will be truncated.
func (p Path) Filename() string {
	_, f := p.Split()
	return f
}

// Split returns the bucket and filename components of the Path.
//
// If a bucket is not defined (doesn't begin with "gs://"), the remainder will
// be considered to be the filename component. If a filename is not defined,
// an empty string will be returned.
func (p Path) Split() (bucket string, filename string) {
	v, ok := trimPrefix(string(p), "gs://")
	if ok {
		// Has a "gs://" prefix, trim that to get the bucket.
		sidx := strings.IndexRune(v, '/')
		if sidx <= 0 {
			// Only a Google Storage bucket name.
			bucket = v
			return
		}

		bucket = v[:sidx]
		v = v[sidx+1:]
	}
	filename = v
	return
}

// IsFullPath returns true if the Path contains both a bucket and file name.
func (p Path) IsFullPath() bool {
	bucket, filename := p.Split()
	return (bucket != "" && filename != "")
}

// Concat concatenates a filename component to the end of Path.
//
// Multiple components may be specified. In this case, each will be added as a
// "/"-delimited component, and will have any present trailing slashes stripped.
func (p Path) Concat(v string, parts ...string) Path {
	comps := make([]string, 0, len(parts)+2)
	add := func(v string) {
		v = stripTrailingSlashes(v)
		if len(v) > 0 {
			comps = append(comps, v)
		}
	}

	// Build our components slice.
	b, f := p.Split()
	add(f)
	add(v)
	for _, p := range parts {
		add(p)
	}
	return MakePath(b, strings.Join(comps, "/"))
}

func trimPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

func stripTrailingSlashes(v string) string {
	return strings.TrimRight(v, "/")
}
