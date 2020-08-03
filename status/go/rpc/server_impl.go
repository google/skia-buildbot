package rpc

// Generate Go structs and Typescript classes from protobuf defintions.
//go:generate protoc --proto_path=${GOPATH}/src:. --twirp_out=. --go_out=. statusFe.proto
//go:generate mv ./go.skia.org/infra/status/go/rpc/statusFe.twirp.go ./statusFe.twirp.go
//go:generate mv ./go.skia.org/infra/status/go/rpc/statusFe.pb.go ./statusFe.pb.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w statusFe.pb.go
//go:generate goimports -w statusFe.twirp.go
//go:generate protoc --twirp_typescript_out=../../modules/rpc statusFe.proto
//go:generate ../../node_modules/typescript-formatter/bin/tsfmt -r ../../modules/rpc/statusFe.ts ../../modules/rpc/twirp.ts

// TODO(westont): Implement a server.