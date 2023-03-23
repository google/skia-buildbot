// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.21.12
// source: analysis.proto

package proto

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// AnalysisMetadata defines the metadata of an analysis.
type AnalysisMetadata struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The report_id of an analysis
	ReportId *string `protobuf:"bytes,1,opt,name=report_id,json=reportId,proto3,oneof" json:"report_id,omitempty"`
}

func (x *AnalysisMetadata) Reset() {
	*x = AnalysisMetadata{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AnalysisMetadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AnalysisMetadata) ProtoMessage() {}

func (x *AnalysisMetadata) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AnalysisMetadata.ProtoReflect.Descriptor instead.
func (*AnalysisMetadata) Descriptor() ([]byte, []int) {
	return file_analysis_proto_rawDescGZIP(), []int{0}
}

func (x *AnalysisMetadata) GetReportId() string {
	if x != nil && x.ReportId != nil {
		return *x.ReportId
	}
	return ""
}

// AnalysisResult defines the result of an analysis
type AnalysisResult struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Analysis result id (PK)
	ResultId string `protobuf:"bytes,1,opt,name=result_id,json=resultId,proto3" json:"result_id,omitempty"`
	// Analysis experiment spec
	ExperimentSpec *ExperimentSpec `protobuf:"bytes,2,opt,name=experiment_spec,json=experimentSpec,proto3" json:"experiment_spec,omitempty"`
	// The metadata of the analysis
	AnalysisMetadata *AnalysisMetadata `protobuf:"bytes,3,opt,name=analysis_metadata,json=analysisMetadata,proto3" json:"analysis_metadata,omitempty"`
	// The calculated statistic of the analysis
	Statistic *Statistic `protobuf:"bytes,4,opt,name=statistic,proto3" json:"statistic,omitempty"`
}

func (x *AnalysisResult) Reset() {
	*x = AnalysisResult{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AnalysisResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AnalysisResult) ProtoMessage() {}

func (x *AnalysisResult) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AnalysisResult.ProtoReflect.Descriptor instead.
func (*AnalysisResult) Descriptor() ([]byte, []int) {
	return file_analysis_proto_rawDescGZIP(), []int{1}
}

func (x *AnalysisResult) GetResultId() string {
	if x != nil {
		return x.ResultId
	}
	return ""
}

func (x *AnalysisResult) GetExperimentSpec() *ExperimentSpec {
	if x != nil {
		return x.ExperimentSpec
	}
	return nil
}

func (x *AnalysisResult) GetAnalysisMetadata() *AnalysisMetadata {
	if x != nil {
		return x.AnalysisMetadata
	}
	return nil
}

func (x *AnalysisResult) GetStatistic() *Statistic {
	if x != nil {
		return x.Statistic
	}
	return nil
}

// Statistic defines the statistic of an analysis
type Statistic struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The lower bound of the analysis result
	Lower float64 `protobuf:"fixed64,1,opt,name=lower,proto3" json:"lower,omitempty"`
	// The upper bound of the analysis result
	Upper float64 `protobuf:"fixed64,2,opt,name=upper,proto3" json:"upper,omitempty"`
	// The P value of the analysis result
	PValue float64 `protobuf:"fixed64,3,opt,name=p_value,json=pValue,proto3" json:"p_value,omitempty"`
	// The defined significance level to calculate the lower and upper bound
	SignificanceLevel float64 `protobuf:"fixed64,4,opt,name=significance_level,json=significanceLevel,proto3" json:"significance_level,omitempty"`
	// The point estimate of the analysis result
	PointEstimate float64 `protobuf:"fixed64,6,opt,name=point_estimate,json=pointEstimate,proto3" json:"point_estimate,omitempty"`
	// The median of control arm of the analysis result
	ControlMedian float64 `protobuf:"fixed64,7,opt,name=control_median,json=controlMedian,proto3" json:"control_median,omitempty"`
	// The median of treatement arm of the analysis result
	TreatmentMedian float64 `protobuf:"fixed64,8,opt,name=treatment_median,json=treatmentMedian,proto3" json:"treatment_median,omitempty"`
}

func (x *Statistic) Reset() {
	*x = Statistic{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Statistic) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Statistic) ProtoMessage() {}

func (x *Statistic) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Statistic.ProtoReflect.Descriptor instead.
func (*Statistic) Descriptor() ([]byte, []int) {
	return file_analysis_proto_rawDescGZIP(), []int{2}
}

func (x *Statistic) GetLower() float64 {
	if x != nil {
		return x.Lower
	}
	return 0
}

func (x *Statistic) GetUpper() float64 {
	if x != nil {
		return x.Upper
	}
	return 0
}

func (x *Statistic) GetPValue() float64 {
	if x != nil {
		return x.PValue
	}
	return 0
}

func (x *Statistic) GetSignificanceLevel() float64 {
	if x != nil {
		return x.SignificanceLevel
	}
	return 0
}

func (x *Statistic) GetPointEstimate() float64 {
	if x != nil {
		return x.PointEstimate
	}
	return 0
}

func (x *Statistic) GetControlMedian() float64 {
	if x != nil {
		return x.ControlMedian
	}
	return 0
}

func (x *Statistic) GetTreatmentMedian() float64 {
	if x != nil {
		return x.TreatmentMedian
	}
	return 0
}

var File_analysis_proto protoreflect.FileDescriptor

var file_analysis_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x0a, 0x63, 0x61, 0x62, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x0a, 0x73, 0x70,
	0x65, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x42, 0x0a, 0x10, 0x41, 0x6e, 0x61, 0x6c,
	0x79, 0x73, 0x69, 0x73, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x20, 0x0a, 0x09,
	0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x48,
	0x00, 0x52, 0x08, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x49, 0x64, 0x88, 0x01, 0x01, 0x42, 0x0c,
	0x0a, 0x0a, 0x5f, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x69, 0x64, 0x22, 0xf2, 0x01, 0x0a,
	0x0e, 0x41, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12,
	0x1b, 0x0a, 0x09, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x49, 0x64, 0x12, 0x43, 0x0a, 0x0f,
	0x65, 0x78, 0x70, 0x65, 0x72, 0x69, 0x6d, 0x65, 0x6e, 0x74, 0x5f, 0x73, 0x70, 0x65, 0x63, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x63, 0x61, 0x62, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2e, 0x45, 0x78, 0x70, 0x65, 0x72, 0x69, 0x6d, 0x65, 0x6e, 0x74, 0x53, 0x70, 0x65,
	0x63, 0x52, 0x0e, 0x65, 0x78, 0x70, 0x65, 0x72, 0x69, 0x6d, 0x65, 0x6e, 0x74, 0x53, 0x70, 0x65,
	0x63, 0x12, 0x49, 0x0a, 0x11, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x5f, 0x6d, 0x65,
	0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x63,
	0x61, 0x62, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x41, 0x6e, 0x61, 0x6c, 0x79, 0x73,
	0x69, 0x73, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x52, 0x10, 0x61, 0x6e, 0x61, 0x6c,
	0x79, 0x73, 0x69, 0x73, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x33, 0x0a, 0x09,
	0x73, 0x74, 0x61, 0x74, 0x69, 0x73, 0x74, 0x69, 0x63, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x15, 0x2e, 0x63, 0x61, 0x62, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x53, 0x74, 0x61,
	0x74, 0x69, 0x73, 0x74, 0x69, 0x63, 0x52, 0x09, 0x73, 0x74, 0x61, 0x74, 0x69, 0x73, 0x74, 0x69,
	0x63, 0x22, 0xf8, 0x01, 0x0a, 0x09, 0x53, 0x74, 0x61, 0x74, 0x69, 0x73, 0x74, 0x69, 0x63, 0x12,
	0x14, 0x0a, 0x05, 0x6c, 0x6f, 0x77, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x01, 0x52, 0x05,
	0x6c, 0x6f, 0x77, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x75, 0x70, 0x70, 0x65, 0x72, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x01, 0x52, 0x05, 0x75, 0x70, 0x70, 0x65, 0x72, 0x12, 0x17, 0x0a, 0x07, 0x70,
	0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x01, 0x52, 0x06, 0x70, 0x56,
	0x61, 0x6c, 0x75, 0x65, 0x12, 0x2d, 0x0a, 0x12, 0x73, 0x69, 0x67, 0x6e, 0x69, 0x66, 0x69, 0x63,
	0x61, 0x6e, 0x63, 0x65, 0x5f, 0x6c, 0x65, 0x76, 0x65, 0x6c, 0x18, 0x04, 0x20, 0x01, 0x28, 0x01,
	0x52, 0x11, 0x73, 0x69, 0x67, 0x6e, 0x69, 0x66, 0x69, 0x63, 0x61, 0x6e, 0x63, 0x65, 0x4c, 0x65,
	0x76, 0x65, 0x6c, 0x12, 0x25, 0x0a, 0x0e, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x5f, 0x65, 0x73, 0x74,
	0x69, 0x6d, 0x61, 0x74, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0d, 0x70, 0x6f, 0x69,
	0x6e, 0x74, 0x45, 0x73, 0x74, 0x69, 0x6d, 0x61, 0x74, 0x65, 0x12, 0x25, 0x0a, 0x0e, 0x63, 0x6f,
	0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x5f, 0x6d, 0x65, 0x64, 0x69, 0x61, 0x6e, 0x18, 0x07, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x0d, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x4d, 0x65, 0x64, 0x69, 0x61,
	0x6e, 0x12, 0x29, 0x0a, 0x10, 0x74, 0x72, 0x65, 0x61, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x5f, 0x6d,
	0x65, 0x64, 0x69, 0x61, 0x6e, 0x18, 0x08, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0f, 0x74, 0x72, 0x65,
	0x61, 0x74, 0x6d, 0x65, 0x6e, 0x74, 0x4d, 0x65, 0x64, 0x69, 0x61, 0x6e, 0x42, 0x21, 0x5a, 0x1f,
	0x67, 0x6f, 0x2e, 0x73, 0x6b, 0x69, 0x61, 0x2e, 0x6f, 0x72, 0x67, 0x2f, 0x69, 0x6e, 0x66, 0x72,
	0x61, 0x2f, 0x63, 0x61, 0x62, 0x65, 0x2f, 0x67, 0x6f, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_analysis_proto_rawDescOnce sync.Once
	file_analysis_proto_rawDescData = file_analysis_proto_rawDesc
)

func file_analysis_proto_rawDescGZIP() []byte {
	file_analysis_proto_rawDescOnce.Do(func() {
		file_analysis_proto_rawDescData = protoimpl.X.CompressGZIP(file_analysis_proto_rawDescData)
	})
	return file_analysis_proto_rawDescData
}

var file_analysis_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_analysis_proto_goTypes = []interface{}{
	(*AnalysisMetadata)(nil), // 0: cabe.proto.AnalysisMetadata
	(*AnalysisResult)(nil),   // 1: cabe.proto.AnalysisResult
	(*Statistic)(nil),        // 2: cabe.proto.Statistic
	(*ExperimentSpec)(nil),   // 3: cabe.proto.ExperimentSpec
}
var file_analysis_proto_depIdxs = []int32{
	3, // 0: cabe.proto.AnalysisResult.experiment_spec:type_name -> cabe.proto.ExperimentSpec
	0, // 1: cabe.proto.AnalysisResult.analysis_metadata:type_name -> cabe.proto.AnalysisMetadata
	2, // 2: cabe.proto.AnalysisResult.statistic:type_name -> cabe.proto.Statistic
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_analysis_proto_init() }
func file_analysis_proto_init() {
	if File_analysis_proto != nil {
		return
	}
	file_spec_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_analysis_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AnalysisMetadata); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_analysis_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AnalysisResult); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_analysis_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Statistic); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_analysis_proto_msgTypes[0].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_analysis_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_analysis_proto_goTypes,
		DependencyIndexes: file_analysis_proto_depIdxs,
		MessageInfos:      file_analysis_proto_msgTypes,
	}.Build()
	File_analysis_proto = out.File
	file_analysis_proto_rawDesc = nil
	file_analysis_proto_goTypes = nil
	file_analysis_proto_depIdxs = nil
}
