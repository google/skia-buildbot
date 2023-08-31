// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=module=go.skia.org/infra/bisection/go/proto --go_out=. --go-grpc_opt=module=go.skia.org/infra/bisection/go/proto --go-grpc_out=.  -I ../../.. bisection/proto/culprit_detection.proto --descriptor_set_in ../../../_bazel_bin/external/com_google_googleapis/google/api/annotations_proto-descriptor-set.proto.bin:../../../_bazel_bin/external/com_google_googleapis/google/api/http_proto-descriptor-set.proto.bin
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w culprit_detection.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w culprit_detection_grpc.pb.go

package proto
