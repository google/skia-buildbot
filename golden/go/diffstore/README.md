To compile the proto buffers definition you need to install version latest
version of protocol buffers.

See https://github.com/google/protobuf/releases for releases and
https://developers.google.com/protocol-buffers/ for documentation.

Install the necessary go packages:

    go get -a github.com/golang/protobuf/protoc-gen-go
    go get -u google.golang.org/grpc

To generate code run in this directory:

    go generate ./...

Or use protoc directly:

    protoc --go_out=plugins=grpc:. diffservice.proto
