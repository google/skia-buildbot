// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=module=go.skia.org/infra/cabe/go/proto --go_out=. --go-grpc_opt=module=go.skia.org/infra/cabe/go/proto --go-grpc_out=.  -I ../../.. cabe/proto/analysis.proto cabe/proto/service.proto cabe/proto/spec.proto
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w analysis.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service_grpc.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w spec.pb.go

package proto
