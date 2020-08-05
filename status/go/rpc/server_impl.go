package rpc

// Generate Go structs and Typescript classes from protobuf defintions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. statusFe.proto
//go:generate mv ./go.skia.org/infra/status/go/rpc/statusFe.twirp.go ./statusFe.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w statusFe.pb.go
//go:generate goimports -w statusFe.twirp.go
//go:Agenerate protoc --proto_path=.:../../.. --twirp_ts_out=../../modules/rpc statusFe.proto
//go:Agenerate ../../node_modules/typescript-formatter/bin/tsfmt -r ../../modules/rpc/status.ts ../../modules/rpc/twirp.ts
//go:Agenerate sed -i 's/\.\/rpc/go.skia.org\/infra\/status\/go\/rpc/g' statusFe.pb.go

// TODO(westont): Implement a server.
 