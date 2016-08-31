// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	// StreamNameSep is the separator rune for stream name tokens.
	StreamNameSep = '/'
	// StreamNameSepStr is a string containing a single rune, StreamNameSep.
	StreamNameSepStr = string(StreamNameSep)

	// StreamPathSep is the separator rune between a stream prefix and its
	// name.
	StreamPathSep = '+'
	// StreamPathSepStr is a string containing a single rune, StreamPathSep.
	StreamPathSepStr = string(StreamPathSep)

	// MaxStreamNameLength is the maximum size, in bytes, of a StreamName. Since
	// stream names must be valid ASCII, this is also the maximum string length.
	MaxStreamNameLength = 4096
)

// StreamName is a structured stream name.
//
// A valid stream name is composed of segments internally separated by a
// StreamNameSep (/).
//
// Each segment:
// - Consists of the following character types:
//   - Alphanumeric characters [a-zA-Z0-9]
//   - Colon (:)
//   - Underscore (_)
//   - Hyphen (-)
//   - Period (.)
// - Must begin with an alphanumeric character.
type StreamName string

func isAlnum(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// Construct builds a path string from a series of individual path components.
// Any leading and trailing separators will be stripped from the components.
//
// The result value will be a valid StreamName if all of the parts are
// valid StreamName strings. Likewise, it may be a valid StreamPath if
// StreamPathSep is one of the parts.
func Construct(parts ...string) string {
	pidx := 0
	for _, v := range parts {
		if v == "" {
			continue
		}
		parts[pidx] = strings.Trim(v, StreamNameSepStr)
		pidx++
	}
	return strings.Join(parts[:pidx], StreamNameSepStr)
}

// MakeStreamName constructs a new stream name from its segments.
//
// This method is guaranteed to return a valid stream name. In order to ensure
// that the arbitrary input can meet this standard, the following
// transformations will be applied as needed:
// - If the segment doesn't begin with an alphanumeric character, the fill
//   string will be prepended.
// - Any character disallowed in the segment will be replaced with an
//   underscore. This includes segment separators within a segment string.
//
// An empty string can be passed for "fill" to just error if a replacement is
// needed.
func MakeStreamName(fill string, s ...string) (StreamName, error) {
	if len(s) == 0 {
		return "", errors.New("at least one segment must be provided")
	}

	fillValidated := false
	for idx, v := range s {
		v = strings.Map(func(r rune) rune {
			switch {
			case r >= 'A' && r <= 'Z':
				fallthrough
			case r >= 'a' && r <= 'z':
				fallthrough
			case r >= '0' && r <= '9':
				fallthrough
			case r == '.':
				fallthrough
			case r == '_':
				fallthrough
			case r == '-':
				fallthrough
			case r == ':':
				return r

			default:
				return '_'
			}
		}, v)
		if r, _ := utf8.DecodeRuneInString(v); !isAlnum(r) {
			// We will prepend the fill sequence to make this a valid stream name.
			// We only need to validate this sequence once.
			if !fillValidated {
				if err := StreamName(fill).Validate(); err != nil {
					return "", fmt.Errorf("fill string must be a valid stream name: %s", err)
				}
				fillValidated = true
			}

			v = fill + v
		}
		s[idx] = v
	}
	result := StreamName(Construct(s...))
	if err := result.Validate(); err != nil {
		return "", err
	}
	return result, nil
}

// String implements flag.String.
func (s *StreamName) String() string {
	return string(*s)
}

// Set implements flag.Value.
func (s *StreamName) Set(value string) error {
	v := StreamName(value)
	if err := v.Validate(); err != nil {
		return err
	}
	*s = v
	return nil
}

// Trim trims separator characters from the beginning and end of a StreamName.
//
// While such a StreamName is not Valid, this method helps correct small user
// input errors.
func (s StreamName) Trim() StreamName {
	return StreamName(trimString(string(s)))
}

// Join concatenates a stream name onto the end of the current name, separating
// it with a separator character.
func (s StreamName) Join(o StreamName) StreamPath {
	return StreamPath(fmt.Sprintf("%s%c%c%c%s",
		s.Trim(), StreamNameSep, StreamPathSep, StreamNameSep, o.Trim()))
}

// Concat constructs a StreamName by concatenating several StreamName components
// together.
func (s StreamName) Concat(o ...StreamName) StreamName {
	parts := make([]string, len(o)+1)
	parts[0] = string(s)
	for i, c := range o {
		parts[i+1] = string(c)
	}
	return StreamName(Construct(parts...))
}

// AsPathPrefix uses s as the prefix component of a StreamPath and constructs
// the remainder of the path with the supplied name.
//
// If name is empty, the resulting path will end in the path separator. For
// example, if s is "foo/bar" and name is "", the path will be "foo/bar/+".
//
// If both s and name are valid StreamNames, this will construct a valid
// StreamPath. If s is a valid StreamName and name is empty, this will construct
// a valid partial StreamPath.
func (s StreamName) AsPathPrefix(name StreamName) StreamPath {
	return StreamPath(Construct(string(s), StreamPathSepStr, string(name)))
}

// Validate tests whether the stream name is valid.
func (s StreamName) Validate() error {
	if len(s) == 0 {
		return errors.New("must contain at least one character")
	}
	if len(s) > MaxStreamNameLength {
		return fmt.Errorf("stream name is too long (%d > %d)", len(s), MaxStreamNameLength)
	}

	var lastRune rune
	var segmentIdx int
	for idx, r := range s {
		// Alphanumeric.
		if !isAlnum(r) {
			// The stream name must begin with an alphanumeric character.
			if idx == segmentIdx {
				return fmt.Errorf("Segment (at %d) must begin with alphanumeric character.", segmentIdx)
			}

			// Test forward slash, and ensure no adjacent forward slashes.
			if r == StreamNameSep {
				segmentIdx = idx + utf8.RuneLen(r)
			} else if !(r == '.' || r == '_' || r == '-' || r == ':') {
				// Test remaining allowed characters.
				return fmt.Errorf("Illegal charater (%c) at index %d.", r, idx)
			}
		}
		lastRune = r
	}

	// The last rune may not be a separator.
	if lastRune == StreamNameSep {
		return errors.New("Name may not end with a separator.")
	}
	return nil
}

// Segments returns the individual StreamName segments by splitting splitting
// the StreamName with StreamNameSep.
func (s StreamName) Segments() []string {
	if len(s) == 0 {
		return nil
	}
	return strings.Split(string(s), string(StreamNameSep))
}

// SegmentCount returns the total number of segments in the StreamName.
func (s StreamName) SegmentCount() int {
	return segmentCount(string(s))
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *StreamName) UnmarshalJSON(data []byte) error {
	v := ""
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if err := StreamName(v).Validate(); err != nil {
		return err
	}
	*s = StreamName(v)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (s StreamName) MarshalJSON() ([]byte, error) {
	v := string(s)
	return json.Marshal(&v)
}

// A StreamPath consists of two StreamName, joined via a StreamPathSep (+)
// separator.
type StreamPath string

// Split splits a StreamPath into its prefix and name components.
//
// If there is no divider present (e.g., foo/bar/baz), the result will parse
// as the stream prefix with an empty name component.
func (p StreamPath) Split() (prefix StreamName, name StreamName) {
	prefix, _, name = p.SplitParts()
	return
}

// SplitParts splits a StreamPath into its prefix and name components.
//
// If there is no separator present (e.g., foo/bar/baz), the result will parse
// as the stream prefix with an empty name component. If there is a separator
// present but no name component, separator will be returned as true with an
// empty name.
func (p StreamPath) SplitParts() (prefix StreamName, sep bool, name StreamName) {
	inside := false
	hasPrefix := false
	segIdx := 0

	for i, r := range p {
		// We have found our separator, and just started iterating the name
		// component.
		if hasPrefix {
			name = StreamName(p[i:])
			return
		}

		switch r {
		case StreamPathSep:
			// If we're at the beginning of a component, this is a potentially-valid
			// separator. We'll identify it as valid if the next character is "/" and
			// break the loop on the character after that.
			sep = !inside
			inside = true

		case StreamNameSep:
			if sep {
				// A component beginning with "+" has been marked, and we have hit
				// another "/", so this is a valid separator. Carve our prefix.
				//
				// We use a separate "hasPrefix" boolean in case the path begin with
				// the separator (e.g., "+/foo").
				prefix = StreamName(p[:segIdx])
				hasPrefix = true
			}
			inside = false
			segIdx = i

		default:
			// Non-special character, mark that we're no longer at the beginning of
			// a component and that this is no longer a valid separator.
			sep = false
			inside = true
		}
	}

	// We finished iterating through our string and didn't find a separator.
	// Either we ended on the separator (prefix is assigned), or we didn't
	// encounter one. In the latter case, the entire path is used as the prefix.
	if !hasPrefix {
		prefix = StreamName(p)
		if sep {
			// If the string ends in a separator, discard and split (e.g., "foo/+").
			prefix = prefix[:segIdx]
		}
	}
	return
}

// Validate checks whether a StreamPath is valid. A valid stream path must have
// a valid prefix and name components.
func (p StreamPath) Validate() error {
	return p.validateImpl(true)
}

// ValidatePartial checks whether a partial StreamPath is valid. A partial
// stream path if appending additional components to it can form a fully-valid
// StreamPath.
func (p StreamPath) ValidatePartial() error {
	return p.validateImpl(false)
}

func (p StreamPath) validateImpl(complete bool) error {
	prefix, name := p.Split()
	if complete || prefix != "" {
		if err := prefix.Validate(); err != nil {
			return err
		}
	}
	if complete || name != "" {
		if err := name.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Trim trims separator characters from the beginning and end of a StreamPath.
//
// While such a StreamPath is not Valid, this method helps correct small user
// input errors.
func (p StreamPath) Trim() StreamPath {
	return StreamPath(trimString(string(p)))
}

// Append returns a StreamPath consisting of the current StreamPath with the
// supplied StreamName appended to the end.
//
// Append will return a valid StreamPath if p and n are both valid.
func (p StreamPath) Append(n string) StreamPath {
	return StreamPath(Construct(string(p), n))
}

// SplitLast splits the rightmost component from a StreamPath, returning the
// intermediate StreamPath.
//
// If the path begins with a leading separator, it will be included in the
// leftmost token. Note that such a path is invalid.
//
// If there are no components in the path, ("", p) will be returned.
func (p StreamPath) SplitLast() (StreamPath, string) {
	if idx := strings.LastIndex(string(p), StreamNameSepStr); idx > 0 {
		return p[:idx], string(p[idx+len(StreamNameSepStr):])
	}
	return "", string(p)
}

// SegmentCount returns the total number of segments in the StreamName.
func (p StreamPath) SegmentCount() int {
	return segmentCount(string(p))
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *StreamPath) UnmarshalJSON(data []byte) error {
	v := ""
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if err := StreamPath(v).Validate(); err != nil {
		return err
	}
	*p = StreamPath(v)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (p StreamPath) MarshalJSON() ([]byte, error) {
	v := string(p)
	return json.Marshal(&v)
}

// StreamNameSlice is a slice of StreamName entries. It implements
// sort.Interface.
type StreamNameSlice []StreamName

var _ sort.Interface = StreamNameSlice(nil)

func (s StreamNameSlice) Len() int           { return len(s) }
func (s StreamNameSlice) Less(i, j int) bool { return s[i] < s[j] }
func (s StreamNameSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func trimString(s string) string {
	for {
		r, l := utf8.DecodeRuneInString(s)
		if r != StreamNameSep {
			break
		}
		s = s[l:]
	}

	for {
		r, l := utf8.DecodeLastRuneInString(s)
		if r != StreamNameSep {
			break
		}
		s = s[:len(s)-l]
	}

	return s
}

func segmentCount(s string) int {
	if len(s) == 0 {
		return 0
	}
	return strings.Count(s, string(StreamNameSep)) + 1
}
