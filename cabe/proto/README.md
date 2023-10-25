# CABE protocol buffer message and grpc service definitions

## API Versioning Strategy

Tl;DR:

- [Follow this advice](https://cloud.google.com/apis/design/versioning).
- CABE's gRPC service and proto package names are versioned like `cabe.v1`, `cabe.v2`, etc...

### Service versioning

CABE uses [semantic versioning](https://semver.org/), however only the major version (`v1`, as of
this writing) is exposed in cabe's proto package name.

Minor version changes and patch version changes (i.e. non-breaking changes) do not require changes
to CABE's proto package name, but any breaking changes _do_ require a major version change, which
means updating the proto package name from `cabe.v<n>` to `cabe.v<n+1>`, and potentially creating
new `./v[n+1]/` paths under this directory to contain the updated `.proto` files.

CABE does not use API
[stability channels](https://cloud.google.com/apis/design/versioning#channel-based_versioning)
for versioning, as it does not have enough distinct users to justify the additional overhead to
implement and maintain them.

### API-specific proto message versioning and Go packages

[AIP 215](https://google.aip.dev/215) offers some guidance on this topic, which CABE should follow.
In particular:

`All protos specific to an API must be within a package with a major version`

Since CABE is implemented in Go, once the API reaches `v2`, the generated Go code for its protos
should include the major version in their Go package name as well. While the current `v1` protos
can declare:

`option go_package = "go.skia.org/infra/cabe/go/proto";`

However, the next major version should instead use:

`option go_package = "go.skia.org/infra/cabe/go/proto/v2";`

and so on.

For any go module, versions `v0` and `v1`, it is not necessary to indlude the major version in the
Go package name. See the official
[Go module documentation](https://go.dev/ref/mod#major-version-suffixes) for more details about
why this is the case.

## Style guide for writing .proto files

[Use the standard protobuf style guide](https://protobuf.dev/programming-guides/style/).

## CABE API Version History

### v1

The CABE gRPC service hosted at cabe.skia.org already had users at the time of its launch and was a
ported from an internally hosted service with the same proto interface. Therefore `v1` is the first
major version for the gRPC API, rather than `v0`.

The proto message and grpc service definitions for `v1` are located in the [./v1](./v1)
subdirectory, and any major API version changes should be defined by files in sibling `./v{2...N}`
subdirectories.
