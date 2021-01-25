package config

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. ./config.proto
//--go:generate mv ./go.skia.org/infra/autoroll/go/config/config.twirp.go ./config.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w config.pb.go
//--go:generate goimports -w config.twirp.go
//--go:generate protoc --twirp_typescript_out=../../modules/config ./config.proto
