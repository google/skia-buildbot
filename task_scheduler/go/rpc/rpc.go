package rpc

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. ./rpc.proto
//go:generate mv ./go.skia.org/infra/task_scheduler/go/rpc/rpc.twirp.go ./rpc.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w rpc.pb.go
//go:generate goimports -w rpc.twirp.go
//go:generate protoc --twirp_typescript_out=../../modules/rpc ./rpc.proto
