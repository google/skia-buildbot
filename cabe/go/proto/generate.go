// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=module=go.skia.org/infra/cabe/go/proto --go_out=. --go-grpc_opt=module=go.skia.org/infra/cabe/go/proto --go-grpc_out=.  -I ../../.. cabe/proto/v1/analysis.proto cabe/proto/v1/service.proto cabe/proto/v1/spec.proto --descriptor_set_in ../../../_bazel_bin/external/com_google_googleapis/google/api/annotations_proto-descriptor-set.proto.bin:../../../_bazel_bin/external/com_google_googleapis/google/api/http_proto-descriptor-set.proto.bin
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w analysis.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service_grpc.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w spec.pb.go

package proto
