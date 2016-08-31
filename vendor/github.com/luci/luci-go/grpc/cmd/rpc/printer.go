// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/luci/luci-go/common/proto/google/descutil"

	"github.com/luci/luci-go/common/data/text/indented"
	"google.golang.org/genproto/protobuf"
)

// printer prints a proto3 definition from a description.
// Does not support options.
type printer struct {
	// File is the file containing the desriptors being printed.
	// Used to relativize names and print comments.
	File *descriptor.FileDescriptorProto
	Out  indented.Writer

	// Err is not nil if writing to Out failed.
	Err error
}

func newPrinter(out io.Writer) *printer {
	return &printer{Out: indented.Writer{Writer: out}}
}

// Printf prints to p.Out unless there was an error.
func (p *printer) Printf(format string, a ...interface{}) {
	if p.Err == nil {
		_, p.Err = fmt.Fprintf(&p.Out, format, a...)
	}
}

// Package prints package declaration.
func (p *printer) Package(name string) {
	p.Printf("package %s;\n", name)
}

// open prints a string, followed by " {\n" and increases indentation level.
// Returns a function that decreases indentation level and closes the brace.
// Usage: defer open("package x")()
func (p *printer) open(format string, a ...interface{}) func() {
	p.Printf(format, a...)
	p.Printf(" {\n")
	p.Out.Level++
	return func() {
		p.Out.Level--
		p.Printf("}\n")
	}
}

// MaybeLeadingComments prints leading comments of the protobuf entity
// at path, if found.
//
// For path, see comment in SourceCodeInfo.Location message in
// common/proto/google/descriptor/descriptor.proto.
func (p *printer) MaybeLeadingComments(path []int) {
	if p.File == nil || p.File.SourceCodeInfo == nil || len(path) == 0 {
		return
	}

	loc := descutil.FindLocation(p.File.SourceCodeInfo, path)
	if loc == nil {
		return
	}

	comments := loc.GetLeadingComments()
	// print comments, but insert "//" before each newline.
	for len(comments) > 0 {
		var toPrint string
		if lineEnd := strings.Index(comments, "\n"); lineEnd >= 0 {
			toPrint = comments[:lineEnd+1] // includes newline
			comments = comments[lineEnd+1:]
		} else {
			// actually this does not happen, because comments always end with
			// newline, but just in case.
			toPrint = comments + "\n"
			comments = ""
		}
		p.Printf("//%s", toPrint)
	}
}

// shorten removes leading "." and trims package name if it matches p.File.
func (p *printer) shorten(name string) string {
	name = strings.TrimPrefix(name, ".")
	if p.File != nil && p.File.GetPackage() != "" {
		name = strings.TrimPrefix(name, p.File.GetPackage()+".")
	}
	return name
}

// Service prints a service definition.
// If methodIndex != -1, only one method is printed.
// If serviceIndex != -1, leading comments are printed if found.
func (p *printer) Service(service *descriptor.ServiceDescriptorProto, serviceIndex, methodIndex int) {
	var path []int
	if serviceIndex != -1 {
		path = []int{descutil.FileDescriptorProtoServiceTag, serviceIndex}
		p.MaybeLeadingComments(path)
	}
	defer p.open("service %s", service.GetName())()

	printMethod := func(i int) {
		var methodPath []int
		if path != nil {
			methodPath = append(path, descutil.ServiceDescriptorProtoMethodTag, i)
		}
		p.Method(service.Method[i], methodPath)
	}

	if methodIndex < 0 {
		for i := range service.Method {
			printMethod(i)
		}
	} else {
		printMethod(methodIndex)
		if len(service.Method) > 1 {
			p.Printf("// other methods were omitted.\n")
		}
	}
}

// Method prints a service method definition.
//
// If path is specified, leading comments are printed if found.
// See also comment in SourceCodeInfo.Location message in
// common/proto/google/descriptor/descriptor.proto.
func (p *printer) Method(method *descriptor.MethodDescriptorProto, path []int) {
	p.MaybeLeadingComments(path)
	p.Printf(
		"rpc %s(%s) returns (%s) {};\n",
		method.GetName(),
		p.shorten(method.GetInputType()),
		p.shorten(method.GetOutputType()),
	)
}

var fieldTypeName = map[descriptor.FieldDescriptorProto_Type]string{
	descriptor.FieldDescriptorProto_TYPE_DOUBLE:   "double",
	descriptor.FieldDescriptorProto_TYPE_FLOAT:    "float",
	descriptor.FieldDescriptorProto_TYPE_INT64:    "int64",
	descriptor.FieldDescriptorProto_TYPE_UINT64:   "uint64",
	descriptor.FieldDescriptorProto_TYPE_INT32:    "int32",
	descriptor.FieldDescriptorProto_TYPE_FIXED64:  "fixed64",
	descriptor.FieldDescriptorProto_TYPE_FIXED32:  "fixed32",
	descriptor.FieldDescriptorProto_TYPE_BOOL:     "bool",
	descriptor.FieldDescriptorProto_TYPE_STRING:   "string",
	descriptor.FieldDescriptorProto_TYPE_BYTES:    "bytes",
	descriptor.FieldDescriptorProto_TYPE_UINT32:   "uint32",
	descriptor.FieldDescriptorProto_TYPE_SFIXED32: "sfixed32",
	descriptor.FieldDescriptorProto_TYPE_SFIXED64: "sfixed64",
	descriptor.FieldDescriptorProto_TYPE_SINT32:   "sint32",
	descriptor.FieldDescriptorProto_TYPE_SINT64:   "sint64",
}

// Field prints a field definition.
//
// If path is specified, leading comments are printed if found.
// See also comment in SourceCodeInfo.Location message in
// common/proto/google/descriptor/descriptor.proto.
func (p *printer) Field(field *descriptor.FieldDescriptorProto, path []int) {
	p.MaybeLeadingComments(path)
	if descutil.Repeated(field) {
		p.Printf("repeated ")
	}

	typeName := fieldTypeName[field.GetType()]
	if typeName == "" {
		typeName = p.shorten(field.GetTypeName())
	}
	if typeName == "" {
		typeName = "<unsupported type>"
	}
	p.Printf("%s %s = %d;\n", typeName, field.GetName(), field.GetNumber())
}

// Message prints a message definition.
//
// If path is specified, leading comments are printed if found.
// See also comment in SourceCodeInfo.Location message in
// common/proto/google/descriptor/descriptor.proto.
func (p *printer) Message(msg *descriptor.DescriptorProto, path []int) {
	p.MaybeLeadingComments(path)
	defer p.open("message %s", msg.GetName())()

	for i := range msg.GetOneofDecl() {
		p.OneOf(msg, i, path)
	}

	for i, f := range msg.Field {
		if f.OneofIndex == nil {
			var fieldPath []int
			if len(path) > 0 {
				fieldPath = append(path, descutil.DescriptorProtoFieldTag, i)
			}
			p.Field(msg.Field[i], fieldPath)
		}
	}
}

// OneOf prints a oneof definition.
//
// If path is specified, leading comments are printed if found.
// See also comment in SourceCodeInfo.Location message in
// common/proto/google/descriptor/descriptor.proto.
func (p *printer) OneOf(msg *descriptor.DescriptorProto, oneOfIndex int, msgPath []int) {
	of := msg.GetOneofDecl()[oneOfIndex]
	if len(msgPath) > 0 {
		p.MaybeLeadingComments(append(msgPath, descutil.DescriptorProtoOneOfTag, oneOfIndex))
	}
	defer p.open("oneof %s", of.GetName())()

	for i, f := range msg.Field {
		if f.OneofIndex != nil && int(f.GetOneofIndex()) == oneOfIndex {
			var fieldPath []int
			if len(msgPath) > 0 {
				fieldPath = append(msgPath, descutil.DescriptorProtoFieldTag, i)
			}
			p.Field(msg.Field[i], fieldPath)
		}
	}
}

// Enum prints an enum definition.
//
// If path is specified, leading comments are printed if found.
// See also comment in SourceCodeInfo.Location message in
// common/proto/google/descriptor/descriptor.proto.
func (p *printer) Enum(enum *descriptor.EnumDescriptorProto, path []int) {
	p.MaybeLeadingComments(path)
	defer p.open("enum %s", enum.GetName())()

	for i, v := range enum.Value {
		var valuePath []int
		if len(path) > 0 {
			valuePath = append(path, descutil.EnumDescriptorProtoValueTag, i)
		}
		p.EnumValue(v, valuePath)
	}
}

// EnumValue prints an enum value definition.
//
// If path is specified, leading comments are printed if found.
// See also comment in SourceCodeInfo.Location message in
// common/proto/google/descriptor/descriptor.proto.
func (p *printer) EnumValue(v *descriptor.EnumValueDescriptorProto, path []int) {
	p.MaybeLeadingComments(path)
	p.Printf("%s = %d;\n", v.GetName(), v.GetNumber())
}
