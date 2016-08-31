// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package hierarchy

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/data/cmpbin"
)

type componentIDType string

const (
	numericStreamID componentIDType = "n"
	textStreamID    componentIDType = "s"
	numericPathID   componentIDType = "y"
	textPathID      componentIDType = "z"
)

// componentID is the datastore ID for a component.
//
// A component represents a single component, and is keyed on the
// component's value and whether the component is a stream or path component.
//
// A log stream path is broken into several components. For example,
// "foo/bar/+/baz/qux" is broken into ["foo", "bar", "+", "baz", "qux"]. The
// intermediate pieces, "foo", "bar", and "baz" are "path components" (e.g.,
// directory name). The terminating component, "qux", is the stream component
// (e.g., file names), since adding it generates a valid stream name.
//
// Note that two streams, "foo/+/bar" and "foo/+/bar/baz", result in components
// "bar". In the first, "bar" is a path component, and in the second "bar" is a
// stream component.
//
// Path component IDs have "~" prepended to them, since "~" is not a valid
// initial stream name character and "~" comes bytewise after all valid
// characters, this will cause paths to sort after streams for a given hierarchy
// level.
type componentID struct {
	// parent is the name of this component's parent.
	parent string
	// name is the name of this component.
	name string
	// stream is true if this is a stream component.
	stream bool
}

var _ ds.PropertyConverter = (*componentID)(nil)

// ToProperty implements ds.PropertyConverter
func (id *componentID) ToProperty() (ds.Property, error) {
	return ds.MkPropertyNI(id.key()), nil
}

// FromProperty implements ds.PropertyConverter
func (id *componentID) FromProperty(p ds.Property) error {
	if p.Type() != ds.PTString {
		return fmt.Errorf("wrong type for property: %s", p.Type())
	}
	return id.setID(p.Value().(string))
}

func (id *componentID) key() string {
	name, numeric := id.maybeNumericID()
	return fmt.Sprintf("%s~%s~%s", id.sortedPrefix(numeric), name, id.parent)
}

func (id *componentID) setID(s string) error {
	parts := strings.SplitN(s, "~", 3)
	if len(parts) != 3 {
		return errors.New("missing minimal key")
	}

	id.name, id.parent = parts[1], parts[2]
	numeric := false
	switch p := componentIDType(parts[0]); p {
	case numericStreamID:
		id.stream = true
		numeric = true

	case textStreamID:
		id.stream = true

	case numericPathID:
		id.stream = false
		numeric = true

	case textPathID:
		id.stream = false

	default:
		return fmt.Errorf("unknown type prefix %q", p)
	}

	if numeric {
		// Split numeric encoding from leading zero count.
		numParts := strings.SplitN(id.name, ":", 2)

		// Render numeric value.
		value, err := decodeCmpbinHexUint(numParts[0])
		if err != nil {
			return fmt.Errorf("failed to decode value: %v", err)
		}

		id.name = strconv.FormatUint(value, 10)

		// Re-add leading zeroes, if any were present.
		if len(numParts) == 2 {
			leadingZeroes, err := decodeCmpbinHexUint(numParts[1])
			if err != nil {
				return fmt.Errorf("failed to decode leading zeroes: %v", err)
			}

			id.name = strings.Repeat("0", int(leadingZeroes)) + id.name
		}
	}

	return nil
}

func (id *componentID) maybeNumericID() (name string, numeric bool) {
	name = id.name

	// Is our name entirely numeric?
	v, err := strconv.ParseUint(name, 10, 64)
	if err != nil {
		return
	}

	// This is a numeric value. Generate a numeric-sortable string.
	//
	// Count the leading zeroes. We do this because the same numeric value can be
	// expressed with different strings, and we need to differentiate them.
	//
	// For example, "000123" vs. "123".
	//
	// We do this by appending ":#", where # is the hex-encoded cmpbin-encoded
	// count of leading zeroes.
	var leadingZeroes uint64
	for _, r := range name {
		if r != '0' {
			break
		}
		leadingZeroes++
	}
	if len(name) > 0 && leadingZeroes == uint64(utf8.RuneCountInString(name)) {
		leadingZeroes--
	}

	var buf bytes.Buffer
	name = encodeCmpbinHexUint(&buf, v)
	if leadingZeroes > 0 {
		name += ":" + encodeCmpbinHexUint(&buf, leadingZeroes)
	}
	numeric = true
	return
}

func (id *componentID) sortedPrefix(numeric bool) componentIDType {
	switch {
	case id.stream && numeric:
		// Numeric stream element.
		return numericStreamID
	case id.stream:
		// Non-numeric stream element.
		return textStreamID
	case numeric:
		// Numeric path component.
		return numericPathID
	default:
		// Non-numeric path component.
		return textPathID
	}
}

func encodeCmpbinHexUint(b *bytes.Buffer, v uint64) string {
	b.Reset()
	if _, err := cmpbin.WriteUint(b, v); err != nil {
		// Cannot happen when writing to a Buffer.
		panic(err)
	}
	return hex.EncodeToString(b.Bytes())
}

func decodeCmpbinHexUint(s string) (uint64, error) {
	buf := make([]byte, hex.DecodedLen(len(s)))
	if _, err := hex.Decode(buf, []byte(s)); err != nil {
		return 0, err
	}

	v, _, err := cmpbin.ReadUint(bytes.NewReader(buf))
	return v, err
}
