// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- -I . -I ../../../.. --descriptor_set_in=../../../../_bazel_bin/external/googleapis+/google/api/annotations_proto-descriptor-set.proto.bin:../../../../_bazel_bin/external/googleapis+/google/api/http_proto-descriptor-set.proto.bin --grpc-gateway_opt=logtostderr=true,paths=source_relative --grpc-gateway_out=. --go_out=paths=source_relative:./ --go-grpc_out=paths=source_relative:./ rag_api.proto
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w rag_api.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w rag_api.pb.gw.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w rag_api_grpc.pb.go

package v1
