// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/logdog/api/logpb"
)

var (
	sizeOfBundleEntryTag   int
	sizeOfLogEntryTag      int
	sizeOfTerminalTag      int
	sizeOfTerminalIndexTag int

	errMalformedProtobufField = errors.New("malformed protobuf field")
)

const (
	// sizeOfBoolTrue is the size of the "true" boolean value.
	sizeOfBoolTrue = 1
)

func init() {
	b := &logpb.ButlerLogBundle{}
	sizeOfBundleEntryTag = mustCalculateTagSize(b, "Entries")

	be := &logpb.ButlerLogBundle_Entry{}
	sizeOfLogEntryTag = mustCalculateTagSize(be, "Logs")
	sizeOfTerminalTag = mustCalculateTagSize(be, "Terminal")
	sizeOfTerminalIndexTag = mustCalculateTagSize(be, "TerminalIndex")
}

func mustCalculateTagSize(i interface{}, field string) int {
	value, err := calculateTagSize(i, field)
	if err != nil {
		panic(err)
	}
	return value
}

func protoSize(m proto.Message) int {
	return proto.Size(m)
}

func calculateTagSize(i interface{}, field string) (int, error) {
	v := reflect.TypeOf(i)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0, fmt.Errorf("sizer: %s is not a struct", v)
	}

	f, ok := v.FieldByName(field)
	if !ok {
		return 0, fmt.Errorf("sizer: could not find field %s.%s", v, field)
	}

	tag, err := protobufTag(f)
	if err != nil {
		return 0, fmt.Errorf("sizer: field %s.%s has no protobuf tag: %s", v, field, err)
	}

	// Protobuf encodes the tag and wire type in the same varint. It does this
	// by allocating three bits for wire type at the base of the tag.
	//
	// https://developers.google.com/protocol-buffers/docs/encoding#structure
	return varintLength(uint64(tag) << 3), nil
}

func varintLength(val uint64) int {
	switch {
	case val == 0:
		return 0
	case val < 0x80:
		return 1
	case val < 0x4000:
		return 2
	case val < 0x200000:
		return 3
	case val < 0x10000000:
		return 4
	case val < 0x800000000:
		return 5
	case val < 0x40000000000:
		return 6
	case val < 0x2000000000000:
		return 7
	case val < 0x100000000000000:
		return 8
	case val < 0x8000000000000000:
		return 9
	default:
		// Maximum uvarint size.
		return 10
	}
}

func protobufTag(f reflect.StructField) (int, error) {
	// If this field doesn't have a "protobuf" tag, ignore it.
	value := f.Tag.Get("protobuf")
	parts := strings.Split(value, ",")
	if len(parts) < 2 {
		return 0, errMalformedProtobufField
	}
	tag, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, errMalformedProtobufField
	}
	return tag, nil
}
