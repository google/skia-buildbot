// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/luci/luci-go/common/proto/google/descutil"

	"github.com/golang/protobuf/proto"

	"google.golang.org/genproto/protobuf"

	. "github.com/smartystreets/goconvey/convey"
)

func TestPrinter(t *testing.T) {
	t.Parallel()

	Convey("Printer", t, func() {
		protoFile, err := ioutil.ReadFile("printer_test.proto")
		So(err, ShouldBeNil)
		protoFileLines := strings.Split(string(protoFile), "\n")

		descFileBytes, err := ioutil.ReadFile("printer_test.desc")
		So(err, ShouldBeNil)

		var desc descriptor.FileDescriptorSet
		err = proto.Unmarshal(descFileBytes, &desc)
		So(err, ShouldBeNil)

		So(desc.File, ShouldHaveLength, 1)
		file := desc.File[0]

		getExpectedDef := func(path ...int) string {
			loc := descutil.FindLocation(file.SourceCodeInfo, path)
			So(loc, ShouldNotBeNil)

			startLine := loc.Span[0]
			endLine := startLine
			if len(loc.Span) > 3 {
				endLine = loc.Span[2]
			}

			for startLine > 0 && strings.HasPrefix(strings.TrimSpace(protoFileLines[startLine-1]), "//") {
				startLine--
			}

			unindent := (len(path) - 2) / 2
			expected := make([]string, endLine-startLine+1)
			for i := 0; i < len(expected); i++ {
				expected[i] = protoFileLines[int(startLine)+i][unindent:]
			}

			return strings.Join(expected, "\n") + "\n"
		}

		var buf bytes.Buffer
		printer := newPrinter(&buf)
		printer.File = file

		checkOutput := func(path ...int) {
			So(buf.String(), ShouldEqual, getExpectedDef(path...))
		}

		Convey("package", func() {
			printer.Package(file.GetPackage())
			checkOutput(descutil.FileDescriptorProtoPackageTag)
		})

		Convey("service", func() {
			for i, s := range file.Service {
				Convey(s.GetName(), func() {
					printer.Service(s, i, -1)
					checkOutput(descutil.FileDescriptorProtoServiceTag, i)
				})
			}
		})

		testEnum := func(e *descriptor.EnumDescriptorProto, path []int) {
			Convey(e.GetName(), func() {
				printer.Enum(e, path)
				checkOutput(path...)
			})
		}

		Convey("enum", func() {
			for i, e := range file.EnumType {
				testEnum(e, []int{descutil.FileDescriptorProtoEnumTag, i})
			}
		})

		Convey("message", func() {
			var testMsg func(*descriptor.DescriptorProto, []int)
			testMsg = func(m *descriptor.DescriptorProto, path []int) {
				Convey(m.GetName(), func() {
					if len(m.NestedType) == 0 && len(m.EnumType) == 0 {
						printer.Message(m, path)
						checkOutput(path...)
					} else {
						for i, m := range m.NestedType {
							testMsg(m, append(path, descutil.DescriptorProtoNestedTypeTag, i))
						}
						for i, e := range m.EnumType {
							testEnum(e, append(path, descutil.DescriptorProtoEnumTypeTag, i))
						}
					}
				})
			}
			for i, m := range file.MessageType {
				testMsg(m, []int{descutil.FileDescriptorProtoMessageTag, i})
			}
		})
	})
}
