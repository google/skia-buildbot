// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package prpc (provisional RPC) implements an RPC client over HTTP 1.x.
//
// Like gRPC:
//  - services are defined in .proto files
//  - service implementation does not depend on pRPC.
// Unlike gRPC:
//  - supports HTTP 1.x and AppEngine 1.x.
//  - does not support streams.
//
// Server
//
// Type Server implements a pRPC server.
//
// Example of usage: https://github.com/luci/luci-go/tree/master/examples/appengine/helloworld_standard
//
// Package discovery implements service discovery.
//
// Compile service definitions
//
// Use cproto tool to compile .proto files to .go files with gRPC and pRPC support.
// It runs protoc and does some additional postprocessing.
//
//  //go:generate cproto
//
// Install cproto:
//  go install github.com/luci/luci-go/grpc/cmd/cproto
//
// Protocol
//
// This section describes the pRPC protocol. It is based on HTTP 1.x and employs
// gRPC codes.
//
// A pRPC server MUST handle HTTP POST requests at `/prpc/{service}/{method}`
// where service is a full service name including full package name.
// The handler must decode an input message from an HTTP request,
// call the service method implementation and
// encode the returned output message or error to the HTTP response.
//
// pRPC protocol defines three protocol buffer encodings and media types.
//  - Binary: "application/prpc; encoding=binary"
//  - JSON:   "application/prpc; encoding=json"   or "application/json"
//    A response body MUST have `)]}'` prefix to avoid CSRF.
//  - Text:   "application/prpc; encoding=text"
// A pRPC server MUST support Binary and SHOULD support JSON and Text.
//
// Request headers:
//  - "X-Prpc-Timeout": specifies request timeout.
//    A client MAY specify it.
//    If a service hits the timeout, a server MUST respond with HTTP 503 and
//    DeadlineExceed gRPC code.
//    Value format: `\d+[HMSmun]` (regular expression), where
//     - H - hour
//     - M - minute
//     - S - second
//     - m - millisecond
//     - u - microsecond
//     - n - nanosecond
//  - "Content-Type": specifies input message encoding in the body.
//    A client SHOULD specify it.
//    If not present, a server MUST treat the input message as Binary.
//  - "Accept": specifies the output message encoding for the response.
//    A client MAY specify it, a server MUST support it.
//  - Any other headers MUST be added to metadata.MD in the context that is
//    passed to the service method implementation.
//    - If a header name has "-Bin" suffix, the server must treat it as
//      base64-encoded and trim the suffix.
//
// Response headers:
//  - "X-Prpc-Grpc-Code": specifies the gRPC code.
//    A server MUST specify it in all cases.
//    - If it is present, the client MUST ignore the HTTP status code.
//      - If OK, the client MUST return the output message
//        decoded from the body.
//      - Otherwise, the client MUST return a gRPC error with the
//        code and message read from the response body.
//    - If not present, the client MUST return a non-gRPC error
//      with message read from the response body.
//      A client MAY read a portion of the response body.
//  - "Content-Type": specifies the output message encoding.
//    A server SHOULD specify it.
//    If not specified, a client MUST treat it is as Binary.
//  - Any metadata returned by a service method implementation MUST go into
//    http headers, unless metadata key starts with "X-Prpc-".
//
// A server MUST always specify "X-Prpc-Grpc-Code".
// The server SHOULD specify HTTP status corresponding to the gRPC code.
//
// If the "X-Prpc-Grpc-Code" response header value is not 0, the response body
// MUST contain a description of the error.
//
// If a service/method is not found, the server MUST respond with Unimplemented
// gRPC code and SHOULD specify HTTP 501 status.
package prpc
