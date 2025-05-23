// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v3.21.12
// source: coverage_service.proto

package v1

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type CoverageRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CoverageRequest) Reset() {
	*x = CoverageRequest{}
	mi := &file_coverage_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CoverageRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CoverageRequest) ProtoMessage() {}

func (x *CoverageRequest) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CoverageRequest.ProtoReflect.Descriptor instead.
func (*CoverageRequest) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{0}
}

type CoverageResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	FileName      *string                `protobuf:"bytes,1,opt,name=file_name,json=fileName,proto3,oneof" json:"file_name,omitempty"`
	BuilderName   *string                `protobuf:"bytes,2,opt,name=builder_name,json=builderName,proto3,oneof" json:"builder_name,omitempty"`
	TestSuiteName []string               `protobuf:"bytes,3,rep,name=test_suite_name,json=testSuiteName,proto3" json:"test_suite_name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CoverageResponse) Reset() {
	*x = CoverageResponse{}
	mi := &file_coverage_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CoverageResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CoverageResponse) ProtoMessage() {}

func (x *CoverageResponse) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CoverageResponse.ProtoReflect.Descriptor instead.
func (*CoverageResponse) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{1}
}

func (x *CoverageResponse) GetFileName() string {
	if x != nil && x.FileName != nil {
		return *x.FileName
	}
	return ""
}

func (x *CoverageResponse) GetBuilderName() string {
	if x != nil && x.BuilderName != nil {
		return *x.BuilderName
	}
	return ""
}

func (x *CoverageResponse) GetTestSuiteName() []string {
	if x != nil {
		return x.TestSuiteName
	}
	return nil
}

type CoverageAllResponses struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Responses     []*CoverageResponse    `protobuf:"bytes,1,rep,name=responses,proto3" json:"responses,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CoverageAllResponses) Reset() {
	*x = CoverageAllResponses{}
	mi := &file_coverage_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CoverageAllResponses) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CoverageAllResponses) ProtoMessage() {}

func (x *CoverageAllResponses) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CoverageAllResponses.ProtoReflect.Descriptor instead.
func (*CoverageAllResponses) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{2}
}

func (x *CoverageAllResponses) GetResponses() []*CoverageResponse {
	if x != nil {
		return x.Responses
	}
	return nil
}

type CoverageListRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	FileName      *string                `protobuf:"bytes,1,opt,name=file_name,json=fileName,proto3,oneof" json:"file_name,omitempty"`
	BuilderName   *string                `protobuf:"bytes,2,opt,name=builder_name,json=builderName,proto3,oneof" json:"builder_name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CoverageListRequest) Reset() {
	*x = CoverageListRequest{}
	mi := &file_coverage_service_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CoverageListRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CoverageListRequest) ProtoMessage() {}

func (x *CoverageListRequest) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CoverageListRequest.ProtoReflect.Descriptor instead.
func (*CoverageListRequest) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{3}
}

func (x *CoverageListRequest) GetFileName() string {
	if x != nil && x.FileName != nil {
		return *x.FileName
	}
	return ""
}

func (x *CoverageListRequest) GetBuilderName() string {
	if x != nil && x.BuilderName != nil {
		return *x.BuilderName
	}
	return ""
}

type CoverageListResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Status        *string                `protobuf:"bytes,1,opt,name=status,proto3,oneof" json:"status,omitempty"`
	TestSuites    []string               `protobuf:"bytes,2,rep,name=test_suites,json=testSuites,proto3" json:"test_suites,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CoverageListResponse) Reset() {
	*x = CoverageListResponse{}
	mi := &file_coverage_service_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CoverageListResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CoverageListResponse) ProtoMessage() {}

func (x *CoverageListResponse) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CoverageListResponse.ProtoReflect.Descriptor instead.
func (*CoverageListResponse) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{4}
}

func (x *CoverageListResponse) GetStatus() string {
	if x != nil && x.Status != nil {
		return *x.Status
	}
	return ""
}

func (x *CoverageListResponse) GetTestSuites() []string {
	if x != nil {
		return x.TestSuites
	}
	return nil
}

type CoverageChangeRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	FileName      *string                `protobuf:"bytes,1,opt,name=file_name,json=fileName,proto3,oneof" json:"file_name,omitempty"`
	BuilderName   *string                `protobuf:"bytes,2,opt,name=builder_name,json=builderName,proto3,oneof" json:"builder_name,omitempty"`
	TestSuiteName []string               `protobuf:"bytes,3,rep,name=test_suite_name,json=testSuiteName,proto3" json:"test_suite_name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CoverageChangeRequest) Reset() {
	*x = CoverageChangeRequest{}
	mi := &file_coverage_service_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CoverageChangeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CoverageChangeRequest) ProtoMessage() {}

func (x *CoverageChangeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CoverageChangeRequest.ProtoReflect.Descriptor instead.
func (*CoverageChangeRequest) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{5}
}

func (x *CoverageChangeRequest) GetFileName() string {
	if x != nil && x.FileName != nil {
		return *x.FileName
	}
	return ""
}

func (x *CoverageChangeRequest) GetBuilderName() string {
	if x != nil && x.BuilderName != nil {
		return *x.BuilderName
	}
	return ""
}

func (x *CoverageChangeRequest) GetTestSuiteName() []string {
	if x != nil {
		return x.TestSuiteName
	}
	return nil
}

type CoverageChangeResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Status        *string                `protobuf:"bytes,1,opt,name=status,proto3,oneof" json:"status,omitempty"`
	Message       *string                `protobuf:"bytes,2,opt,name=message,proto3,oneof" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CoverageChangeResponse) Reset() {
	*x = CoverageChangeResponse{}
	mi := &file_coverage_service_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CoverageChangeResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CoverageChangeResponse) ProtoMessage() {}

func (x *CoverageChangeResponse) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CoverageChangeResponse.ProtoReflect.Descriptor instead.
func (*CoverageChangeResponse) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{6}
}

func (x *CoverageChangeResponse) GetStatus() string {
	if x != nil && x.Status != nil {
		return *x.Status
	}
	return ""
}

func (x *CoverageChangeResponse) GetMessage() string {
	if x != nil && x.Message != nil {
		return *x.Message
	}
	return ""
}

type TestSuite struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	TestSuiteName *string                `protobuf:"bytes,1,opt,name=test_suite_name,json=testSuiteName,proto3,oneof" json:"test_suite_name,omitempty"`
	TestCaseName  []string               `protobuf:"bytes,2,rep,name=test_case_name,json=testCaseName,proto3" json:"test_case_name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TestSuite) Reset() {
	*x = TestSuite{}
	mi := &file_coverage_service_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TestSuite) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestSuite) ProtoMessage() {}

func (x *TestSuite) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestSuite.ProtoReflect.Descriptor instead.
func (*TestSuite) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{7}
}

func (x *TestSuite) GetTestSuiteName() string {
	if x != nil && x.TestSuiteName != nil {
		return *x.TestSuiteName
	}
	return ""
}

func (x *TestSuite) GetTestCaseName() []string {
	if x != nil {
		return x.TestCaseName
	}
	return nil
}

type Builder struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Name of the builder.
	BuilderName *string `protobuf:"bytes,1,opt,name=builder_name,json=builderName,proto3,oneof" json:"builder_name,omitempty"`
	// Architecture of the builder.
	Architecture *string `protobuf:"bytes,2,opt,name=architecture,proto3,oneof" json:"architecture,omitempty"`
	// OS of the builder.
	Os *string `protobuf:"bytes,3,opt,name=os,proto3,oneof" json:"os,omitempty"`
	// Test suites that are part of the builder.
	TestSuite     []*TestSuite `protobuf:"bytes,4,rep,name=test_suite,json=testSuite,proto3" json:"test_suite,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Builder) Reset() {
	*x = Builder{}
	mi := &file_coverage_service_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Builder) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Builder) ProtoMessage() {}

func (x *Builder) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Builder.ProtoReflect.Descriptor instead.
func (*Builder) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{8}
}

func (x *Builder) GetBuilderName() string {
	if x != nil && x.BuilderName != nil {
		return *x.BuilderName
	}
	return ""
}

func (x *Builder) GetArchitecture() string {
	if x != nil && x.Architecture != nil {
		return *x.Architecture
	}
	return ""
}

func (x *Builder) GetOs() string {
	if x != nil && x.Os != nil {
		return *x.Os
	}
	return ""
}

func (x *Builder) GetTestSuite() []*TestSuite {
	if x != nil {
		return x.TestSuite
	}
	return nil
}

type TestSuiteMap struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Name of the file.
	FileName *string `protobuf:"bytes,1,opt,name=file_name,json=fileName,proto3,oneof" json:"file_name,omitempty"`
	// Name of the builder.
	BuilderName *string `protobuf:"bytes,2,opt,name=builder_name,json=builderName,proto3,oneof" json:"builder_name,omitempty"`
	// Name of test suite.
	TestSuiteName []string `protobuf:"bytes,3,rep,name=test_suite_name,json=testSuiteName,proto3" json:"test_suite_name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TestSuiteMap) Reset() {
	*x = TestSuiteMap{}
	mi := &file_coverage_service_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TestSuiteMap) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TestSuiteMap) ProtoMessage() {}

func (x *TestSuiteMap) ProtoReflect() protoreflect.Message {
	mi := &file_coverage_service_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TestSuiteMap.ProtoReflect.Descriptor instead.
func (*TestSuiteMap) Descriptor() ([]byte, []int) {
	return file_coverage_service_proto_rawDescGZIP(), []int{9}
}

func (x *TestSuiteMap) GetFileName() string {
	if x != nil && x.FileName != nil {
		return *x.FileName
	}
	return ""
}

func (x *TestSuiteMap) GetBuilderName() string {
	if x != nil && x.BuilderName != nil {
		return *x.BuilderName
	}
	return ""
}

func (x *TestSuiteMap) GetTestSuiteName() []string {
	if x != nil {
		return x.TestSuiteName
	}
	return nil
}

var File_coverage_service_proto protoreflect.FileDescriptor

const file_coverage_service_proto_rawDesc = "" +
	"\n" +
	"\x16coverage_service.proto\x12\vcoverage.v1\"\x11\n" +
	"\x0fCoverageRequest\"\xa3\x01\n" +
	"\x10CoverageResponse\x12 \n" +
	"\tfile_name\x18\x01 \x01(\tH\x00R\bfileName\x88\x01\x01\x12&\n" +
	"\fbuilder_name\x18\x02 \x01(\tH\x01R\vbuilderName\x88\x01\x01\x12&\n" +
	"\x0ftest_suite_name\x18\x03 \x03(\tR\rtestSuiteNameB\f\n" +
	"\n" +
	"_file_nameB\x0f\n" +
	"\r_builder_name\"S\n" +
	"\x14CoverageAllResponses\x12;\n" +
	"\tresponses\x18\x01 \x03(\v2\x1d.coverage.v1.CoverageResponseR\tresponses\"~\n" +
	"\x13CoverageListRequest\x12 \n" +
	"\tfile_name\x18\x01 \x01(\tH\x00R\bfileName\x88\x01\x01\x12&\n" +
	"\fbuilder_name\x18\x02 \x01(\tH\x01R\vbuilderName\x88\x01\x01B\f\n" +
	"\n" +
	"_file_nameB\x0f\n" +
	"\r_builder_name\"_\n" +
	"\x14CoverageListResponse\x12\x1b\n" +
	"\x06status\x18\x01 \x01(\tH\x00R\x06status\x88\x01\x01\x12\x1f\n" +
	"\vtest_suites\x18\x02 \x03(\tR\n" +
	"testSuitesB\t\n" +
	"\a_status\"\xa8\x01\n" +
	"\x15CoverageChangeRequest\x12 \n" +
	"\tfile_name\x18\x01 \x01(\tH\x00R\bfileName\x88\x01\x01\x12&\n" +
	"\fbuilder_name\x18\x02 \x01(\tH\x01R\vbuilderName\x88\x01\x01\x12&\n" +
	"\x0ftest_suite_name\x18\x03 \x03(\tR\rtestSuiteNameB\f\n" +
	"\n" +
	"_file_nameB\x0f\n" +
	"\r_builder_name\"k\n" +
	"\x16CoverageChangeResponse\x12\x1b\n" +
	"\x06status\x18\x01 \x01(\tH\x00R\x06status\x88\x01\x01\x12\x1d\n" +
	"\amessage\x18\x02 \x01(\tH\x01R\amessage\x88\x01\x01B\t\n" +
	"\a_statusB\n" +
	"\n" +
	"\b_message\"r\n" +
	"\tTestSuite\x12+\n" +
	"\x0ftest_suite_name\x18\x01 \x01(\tH\x00R\rtestSuiteName\x88\x01\x01\x12$\n" +
	"\x0etest_case_name\x18\x02 \x03(\tR\ftestCaseNameB\x12\n" +
	"\x10_test_suite_name\"\xcf\x01\n" +
	"\aBuilder\x12&\n" +
	"\fbuilder_name\x18\x01 \x01(\tH\x00R\vbuilderName\x88\x01\x01\x12'\n" +
	"\farchitecture\x18\x02 \x01(\tH\x01R\farchitecture\x88\x01\x01\x12\x13\n" +
	"\x02os\x18\x03 \x01(\tH\x02R\x02os\x88\x01\x01\x125\n" +
	"\n" +
	"test_suite\x18\x04 \x03(\v2\x16.coverage.v1.TestSuiteR\ttestSuiteB\x0f\n" +
	"\r_builder_nameB\x0f\n" +
	"\r_architectureB\x05\n" +
	"\x03_os\"\x9f\x01\n" +
	"\fTestSuiteMap\x12 \n" +
	"\tfile_name\x18\x01 \x01(\tH\x00R\bfileName\x88\x01\x01\x12&\n" +
	"\fbuilder_name\x18\x02 \x01(\tH\x01R\vbuilderName\x88\x01\x01\x12&\n" +
	"\x0ftest_suite_name\x18\x03 \x03(\tR\rtestSuiteNameB\f\n" +
	"\n" +
	"_file_nameB\x0f\n" +
	"\r_builder_name2\xec\x02\n" +
	"\x0fCoverageService\x12P\n" +
	"\vGetAllFiles\x12\x1c.coverage.v1.CoverageRequest\x1a!.coverage.v1.CoverageAllResponses\"\x00\x12U\n" +
	"\fGetTestSuite\x12 .coverage.v1.CoverageListRequest\x1a!.coverage.v1.CoverageListResponse\"\x00\x12W\n" +
	"\n" +
	"InsertFile\x12\".coverage.v1.CoverageChangeRequest\x1a#.coverage.v1.CoverageChangeResponse\"\x00\x12W\n" +
	"\n" +
	"DeleteFile\x12\".coverage.v1.CoverageChangeRequest\x1a#.coverage.v1.CoverageChangeResponse\"\x00B(Z&go.skia.org/infra/go/coverage/proto/v1b\x06proto3"

var (
	file_coverage_service_proto_rawDescOnce sync.Once
	file_coverage_service_proto_rawDescData []byte
)

func file_coverage_service_proto_rawDescGZIP() []byte {
	file_coverage_service_proto_rawDescOnce.Do(func() {
		file_coverage_service_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_coverage_service_proto_rawDesc), len(file_coverage_service_proto_rawDesc)))
	})
	return file_coverage_service_proto_rawDescData
}

var file_coverage_service_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_coverage_service_proto_goTypes = []any{
	(*CoverageRequest)(nil),        // 0: coverage.v1.CoverageRequest
	(*CoverageResponse)(nil),       // 1: coverage.v1.CoverageResponse
	(*CoverageAllResponses)(nil),   // 2: coverage.v1.CoverageAllResponses
	(*CoverageListRequest)(nil),    // 3: coverage.v1.CoverageListRequest
	(*CoverageListResponse)(nil),   // 4: coverage.v1.CoverageListResponse
	(*CoverageChangeRequest)(nil),  // 5: coverage.v1.CoverageChangeRequest
	(*CoverageChangeResponse)(nil), // 6: coverage.v1.CoverageChangeResponse
	(*TestSuite)(nil),              // 7: coverage.v1.TestSuite
	(*Builder)(nil),                // 8: coverage.v1.Builder
	(*TestSuiteMap)(nil),           // 9: coverage.v1.TestSuiteMap
}
var file_coverage_service_proto_depIdxs = []int32{
	1, // 0: coverage.v1.CoverageAllResponses.responses:type_name -> coverage.v1.CoverageResponse
	7, // 1: coverage.v1.Builder.test_suite:type_name -> coverage.v1.TestSuite
	0, // 2: coverage.v1.CoverageService.GetAllFiles:input_type -> coverage.v1.CoverageRequest
	3, // 3: coverage.v1.CoverageService.GetTestSuite:input_type -> coverage.v1.CoverageListRequest
	5, // 4: coverage.v1.CoverageService.InsertFile:input_type -> coverage.v1.CoverageChangeRequest
	5, // 5: coverage.v1.CoverageService.DeleteFile:input_type -> coverage.v1.CoverageChangeRequest
	2, // 6: coverage.v1.CoverageService.GetAllFiles:output_type -> coverage.v1.CoverageAllResponses
	4, // 7: coverage.v1.CoverageService.GetTestSuite:output_type -> coverage.v1.CoverageListResponse
	6, // 8: coverage.v1.CoverageService.InsertFile:output_type -> coverage.v1.CoverageChangeResponse
	6, // 9: coverage.v1.CoverageService.DeleteFile:output_type -> coverage.v1.CoverageChangeResponse
	6, // [6:10] is the sub-list for method output_type
	2, // [2:6] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_coverage_service_proto_init() }
func file_coverage_service_proto_init() {
	if File_coverage_service_proto != nil {
		return
	}
	file_coverage_service_proto_msgTypes[1].OneofWrappers = []any{}
	file_coverage_service_proto_msgTypes[3].OneofWrappers = []any{}
	file_coverage_service_proto_msgTypes[4].OneofWrappers = []any{}
	file_coverage_service_proto_msgTypes[5].OneofWrappers = []any{}
	file_coverage_service_proto_msgTypes[6].OneofWrappers = []any{}
	file_coverage_service_proto_msgTypes[7].OneofWrappers = []any{}
	file_coverage_service_proto_msgTypes[8].OneofWrappers = []any{}
	file_coverage_service_proto_msgTypes[9].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_coverage_service_proto_rawDesc), len(file_coverage_service_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_coverage_service_proto_goTypes,
		DependencyIndexes: file_coverage_service_proto_depIdxs,
		MessageInfos:      file_coverage_service_proto_msgTypes,
	}.Build()
	File_coverage_service_proto = out.File
	file_coverage_service_proto_goTypes = nil
	file_coverage_service_proto_depIdxs = nil
}
