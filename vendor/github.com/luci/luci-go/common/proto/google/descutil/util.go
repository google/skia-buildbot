// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package descutil

import (
	"strings"

	pb "google.golang.org/genproto/protobuf"
)

// splitFullName splits package name and service/type name.
func splitFullName(fullName string) (pkg string, name string) {
	lastDot := strings.LastIndex(fullName, ".")
	if lastDot < 0 {
		return "", fullName
	}
	return fullName[:lastDot], fullName[lastDot+1:]
}

////////////////////////////////////////////////////////////////////////////////
// FileDescriptorSet

// FindFile searches for a FileDescriptorProto by name.
func FindFile(s *pb.FileDescriptorSet, name string) int {
	for i, x := range s.GetFile() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

// Resolve searches for an object by full name. obj can be one of
// *ServiceDescriptorProto,
// *MethodDescriptorProto,
// *DescriptorProto,
// *FieldDescriptorProto,
// *DescriptorProto,
// *EnumDescriptorProto,
// *EnumValueDescriptorProto or
// nil
//
// For path, see comment in SourceCodeInfo message.
func Resolve(s *pb.FileDescriptorSet, fullName string) (file *pb.FileDescriptorProto, obj interface{}, path []int) {
	if fullName == "" {
		return nil, nil, nil
	}
	pkg, name := splitFullName(fullName)

	// Check top-level objects.
	for _, f := range s.GetFile() {
		if f.GetPackage() == pkg {
			if i := FindServiceForFile(f, name); i != -1 {
				return f, f.Service[i], []int{FileDescriptorProtoServiceTag, i}
			}
			if i := FindMessageForFile(f, name); i != -1 {
				return f, f.MessageType[i], []int{FileDescriptorProtoMessageTag, i}
			}
			if i := FindEnumForFile(f, name); i != -1 {
				return f, f.EnumType[i], []int{FileDescriptorProtoEnumTag, i}
			}
		}
	}

	// Recurse.
	var parent interface{}
	file, parent, path = Resolve(s, pkg)
	switch parent := parent.(type) {

	case *pb.ServiceDescriptorProto:
		if i := FindMethodForService(parent, name); i != -1 {
			return file, parent.Method[i], append(path, ServiceDescriptorProtoMethodTag, i)
		}

	case *pb.DescriptorProto:
		if i := FindMessage(parent, name); i != -1 {
			return file, parent.NestedType[i], append(path, DescriptorProtoNestedTypeTag, i)
		}
		if i := FindEnum(parent, name); i != -1 {
			return file, parent.EnumType[i], append(path, DescriptorProtoEnumTypeTag, i)
		}
		if i := FindField(parent, name); i != -1 {
			return file, parent.Field[i], append(path, DescriptorProtoFieldTag, i)
		}
		if i := FindOneOf(parent, name); i != -1 {
			return file, parent.OneofDecl[i], append(path, DescriptorProtoOneOfTag, i)
		}

	case *pb.EnumDescriptorProto:
		if i := FindEnumValue(parent, name); i != -1 {
			return file, parent.Value[i], append(path, EnumDescriptorProtoValueTag, i)
		}
	}

	return nil, nil, nil
}

// FindService searches for a service by full name.
func FindService(s *pb.FileDescriptorSet, fullName string) (file *pb.FileDescriptorProto, serviceIndex int) {
	pkg, name := splitFullName(fullName)
	for _, f := range s.GetFile() {
		if f.GetPackage() == pkg {
			if i := FindServiceForFile(f, name); i != -1 {
				return f, i
			}
		}
	}
	return nil, -1
}

////////////////////////////////////////////////////////////////////////////////
// FileDescriptorProto

// FindServiceForFile searches for a FileDescriptorProto by name.
func FindServiceForFile(f *pb.FileDescriptorProto, name string) int {
	for i, x := range f.GetService() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

// FindMessageForFile searches for a DescriptorProto by name.
func FindMessageForFile(f *pb.FileDescriptorProto, name string) int {
	for i, x := range f.GetMessageType() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

// FindEnumForFile searches for an EnumDescriptorProto by name.
func FindEnumForFile(f *pb.FileDescriptorProto, name string) int {
	for i, x := range f.GetEnumType() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

////////////////////////////////////////////////////////////////////////////////
// ServiceDescriptorProto

// FindMethodForService searches for a MethodDescriptorProto by name.
func FindMethodForService(s *pb.ServiceDescriptorProto, name string) int {
	for i, x := range s.GetMethod() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

////////////////////////////////////////////////////////////////////////////////
// DescriptorProto (a message)

// FindField searches for a FieldDescriptorProto by name.
func FindField(d *pb.DescriptorProto, name string) int {
	for i, x := range d.GetField() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

// FindMessage searches for a nested DescriptorProto by name.
func FindMessage(d *pb.DescriptorProto, name string) int {
	for i, x := range d.GetNestedType() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

// FindEnum searches for a nested EnumDescriptorProto by name.
func FindEnum(d *pb.DescriptorProto, name string) int {
	for i, x := range d.GetEnumType() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

// FindOneOf searches for a nested OneofDescriptorProto by name.
func FindOneOf(d *pb.DescriptorProto, name string) int {
	for i, x := range d.GetOneofDecl() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

////////////////////////////////////////////////////////////////////////////////
// FieldDescriptorProto

// Repeated returns true if the field is repeated.
func Repeated(f *pb.FieldDescriptorProto) bool {
	return f.GetLabel() == pb.FieldDescriptorProto_LABEL_REPEATED
}

////////////////////////////////////////////////////////////////////////////////
// EnumDescriptorProto

// FindEnumValue searches for an EnumValueDescriptorProto by name.
func FindEnumValue(e *pb.EnumDescriptorProto, name string) int {
	for i, x := range e.GetValue() {
		if x.GetName() == name {
			return i
		}
	}
	return -1
}

// FindValueByNumber searches for an EnumValueDescriptorProto by number.
func FindValueByNumber(e *pb.EnumDescriptorProto, number int32) int {
	for i, x := range e.GetValue() {
		if x.GetNumber() == number {
			return i
		}
	}
	return -1
}

////////////////////////////////////////////////////////////////////////////////
// SourceCodeInfo

// FindLocation searches for a location by path.
func FindLocation(s *pb.SourceCodeInfo, path []int) *pb.SourceCodeInfo_Location {
Outer:
	for _, l := range s.GetLocation() {
		if len(path) != len(l.Path) {
			continue
		}
		for i := range path {
			if int32(path[i]) != l.Path[i] {
				continue Outer
			}
		}
		return l
	}
	return nil
}
