// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=module=go.skia.org/infra/perf/go/anomalygroup/proto/v1 --go_out=. --go-grpc_opt=module=go.skia.org/infra/perf/go/anomalygroup/proto/v1 --go-grpc_out=.  -I . ./service.proto
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w service_grpc.pb.go

package v1
