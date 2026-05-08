// Generate the go code from the protocol buffer definitions.
// To re-generate definitions run the command below from the `buildbot` directory:
// BUILD_WORKSPACE_DIRECTORY=$PWD go generate ./pinpoint/proto/v1/...

//go:generate bazelisk run --config=mayberemote //:protoc -- -I . -I "${BUILD_WORKSPACE_DIRECTORY}" --descriptor_set_in=$BUILD_WORKSPACE_DIRECTORY/_bazel_bin/external/googleapis+/google/api/annotations_proto-descriptor-set.proto.bin:$BUILD_WORKSPACE_DIRECTORY/_bazel_bin/external/googleapis+/google/api/http_proto-descriptor-set.proto.bin "--grpc-gateway_opt=paths=source_relative" --grpc-gateway_out=. --go_opt=paths=source_relative --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative ./service.proto
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service.pb.gw.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service_grpc.pb.go
//go:generate bazelisk run --config=mayberemote //:protoc -- -I . -I "${BUILD_WORKSPACE_DIRECTORY}" --descriptor_set_in=$BUILD_WORKSPACE_DIRECTORY/_bazel_bin/external/googleapis+/google/api/annotations_proto-descriptor-set.proto.bin:$BUILD_WORKSPACE_DIRECTORY/_bazel_bin/external/googleapis+/google/api/http_proto-descriptor-set.proto.bin:$BUILD_WORKSPACE_DIRECTORY/_bazel_bin/external/protoc-gen-validate+/validate/validate_proto-descriptor-set.proto.bin "--grpc-gateway_opt=paths=source_relative" --grpc-gateway_out=. --go_opt=paths=source_relative --go_out=. --go-grpc_out=. --go-grpc_opt=paths=source_relative ./gateway.proto
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w gateway.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w gateway.pb.gw.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w gateway_grpc.pb.go

package pinpointpb
