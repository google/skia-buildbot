package types

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_opt=paths=source_relative --go_out=. ./rpc.proto
//go:generate goimports -w rpc.pb.go

import "errors"

var (
	ErrUnknownId = errors.New("Unknown ID")
)
