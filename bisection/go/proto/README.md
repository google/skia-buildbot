# Pinpoint Protocol buffer definitions

This directory contains `.pb.go` and `_grpc.pb.go` files generated from
the `.proto` files in //bisection/proto/...

# How to update protos

To update protobuf definitions:

- make your edits to the `.proto` files(s) in //bisection/proto/...
- run `go generate` from this directory to update the generated `.go` files
- test and send your changes for review

# Gotchas

## Do not use the `optional` keyword.

`optional` is no longer a keyword in proto3, since fields are optional
by default now.

The `go generate` step here appears to be perfectly fine if .proto files
contain the `optional` keyword, but these files all use `proto3` syntax.
Other protoc invocations (such as the ones used to generate the
python stubs for cabe in chromeperf's codebase) have had problems with
`optional` and fail with error messages about it.

So `optional` is unnecessary in proto3, and it breaks things elsewhere if
you include it here, so please don't.

## Do not use the `required` keyword.

go/required-fields-harmful
