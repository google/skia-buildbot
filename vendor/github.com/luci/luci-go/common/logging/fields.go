// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logging

import (
	"bytes"
	"fmt"
	"sort"

	"golang.org/x/net/context"
)

const (
	// ErrorKey is a logging field key to use for errors.
	ErrorKey = "error"
)

// Fields maps string keys to arbitrary values.
//
// Fields can be added to a Context. Fields added to a Context augment those
// in the Context's parent Context, overriding duplicate keys. When Fields are
// added to a Context, they are copied internally for retention.
//
// Fields can also be added directly to a log message by calling its
// logging passthrough methods. This immediate usage avoids the overhead of
// duplicating the fields for retention.
type Fields map[string]interface{}

// NewFields instantiates a new Fields instance by duplicating the supplied map.
func NewFields(v map[string]interface{}) Fields {
	fields := make(Fields)
	for k, v := range v {
		fields[k] = v
	}
	return fields
}

// WithError returns a Fields instance containing an error.
func WithError(err error) Fields {
	return Fields{
		ErrorKey: err,
	}
}

// Copy returns a copy of this Fields with the keys from other overlaid on top
// of this one's.
func (f Fields) Copy(other Fields) Fields {
	if len(f) == 0 && len(other) == 0 {
		return nil
	}

	ret := make(Fields, len(f)+len(other))
	for k, v := range f {
		ret[k] = v
	}
	for k, v := range other {
		ret[k] = v
	}
	return ret
}

// SortedEntries processes a Fields object, pruning invisible fields, placing
// the ErrorKey field first, and then sorting the remaining fields by key.
func (f Fields) SortedEntries() (s []*FieldEntry) {
	if len(f) == 0 {
		return nil
	}
	s = make([]*FieldEntry, 0, len(f))
	for k, v := range f {
		s = append(s, &FieldEntry{k, v})
	}
	sort.Sort(fieldEntrySlice(s))
	return
}

// String returns a string describing the contents of f in a sorted,
// dictionary-like format.
func (f Fields) String() string {
	b := bytes.Buffer{}
	b.WriteRune('{')
	for idx, e := range f.SortedEntries() {
		if idx > 0 {
			b.WriteString(", ")
		}
		b.WriteString(e.String())
	}
	b.WriteRune('}')
	return b.String()
}

// Debugf is a shorthand method to call the current logger's Errorf method.
func (f Fields) Debugf(c context.Context, fmt string, args ...interface{}) {
	Get(SetFields(c, f)).LogCall(Debug, 1, fmt, args)
}

// Infof is a shorthand method to call the current logger's Errorf method.
func (f Fields) Infof(c context.Context, fmt string, args ...interface{}) {
	Get(SetFields(c, f)).LogCall(Info, 1, fmt, args)
}

// Warningf is a shorthand method to call the current logger's Errorf method.
func (f Fields) Warningf(c context.Context, fmt string, args ...interface{}) {
	Get(SetFields(c, f)).LogCall(Warning, 1, fmt, args)
}

// Errorf is a shorthand method to call the current logger's Errorf method.
func (f Fields) Errorf(c context.Context, fmt string, args ...interface{}) {
	Get(SetFields(c, f)).LogCall(Error, 1, fmt, args)
}

// FieldEntry is a static representation of a single key/value entry in a
// Fields.
type FieldEntry struct {
	Key   string      // The field's key.
	Value interface{} // The field's value.
}

// String returns the string representation of the field entry:
// "<key>":"<value>".
func (e *FieldEntry) String() string {
	value := e.Value
	if s, ok := value.(fmt.Stringer); ok {
		value = s.String()
	}

	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q:%q", e.Key, v)

	case error:
		return fmt.Sprintf("%q:%q", e.Key, v.Error())

	default:
		return fmt.Sprintf("%q:%#v", e.Key, v)
	}
}

// fieldEntrySlice is a slice of FieldEntry which implements sort.Interface.
// The error field is placed before any other field; the remaining fields are
// sorted alphabetically.
type fieldEntrySlice []*FieldEntry

var _ sort.Interface = fieldEntrySlice(nil)

func (s fieldEntrySlice) Less(i, j int) bool {
	if s[i].Key == ErrorKey {
		return s[j].Key != ErrorKey
	}
	if s[j].Key == ErrorKey {
		return false
	}
	return s[i].Key < s[j].Key
}

func (s fieldEntrySlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s fieldEntrySlice) Len() int {
	return len(s)
}

// SetFields adds the additional fields as context for the current Logger. The
// display of these fields depends on the implementation of the Logger. The
// new context will contain the combination of its current Fields, updated with
// the new ones (see Fields.Copy). Specifying the new fields as nil will
// clear the currently set fields.
func SetFields(c context.Context, fields Fields) context.Context {
	return context.WithValue(c, fieldsKey, GetFields(c).Copy(fields))
}

// SetField is a convenience method for SetFields for a single key/value
// pair.
func SetField(c context.Context, key string, value interface{}) context.Context {
	return SetFields(c, Fields{key: value})
}

// GetFields returns the current Fields.
//
// This method is used for logger implementations with the understanding that
// the returned fields must not be mutated.
func GetFields(c context.Context) Fields {
	if ret, ok := c.Value(fieldsKey).(Fields); ok {
		return ret
	}
	return nil
}
