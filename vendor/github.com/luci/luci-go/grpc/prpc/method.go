// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package prpc

import (
	"net/http"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/luci-go/grpc/grpcutil"
)

type method struct {
	service *service
	desc    grpc.MethodDesc
}

// handle decodes an input protobuf message from the HTTP request,
// delegates RPC handling to the inner implementation and
// encodes the output message back to the HTTP response.
//
// If the inner handler returns an error, HTTP status is determined using
// errorStatus.
// Prints only "Internal server error" if the code is Internal.
// Logs the error if code is Internal or Unknown.
func (m *method) handle(c context.Context, w http.ResponseWriter, r *http.Request, unaryInt grpc.UnaryServerInterceptor) *response {
	defer r.Body.Close()

	format, perr := responseFormat(r.Header.Get(headerAccept))
	if perr != nil {
		return respondProtocolError(perr)
	}

	c, err := parseHeader(c, r.Header)
	if err != nil {
		return respondProtocolError(withStatus(err, http.StatusBadRequest))
	}

	out, err := m.desc.Handler(m.service.impl, c, func(in interface{}) error {
		if in == nil {
			return grpcutil.Errf(codes.Internal, "input message is nil")
		}
		// Do not collapse it to one line. There is implicit err type conversion.
		if perr := readMessage(r, in.(proto.Message)); perr != nil {
			return perr
		}
		return nil
	}, unaryInt)
	if err != nil {
		if perr, ok := err.(*protocolError); ok {
			return respondProtocolError(perr)
		}
		return errResponse(errorCode(err), 0, escapeFmt(grpc.ErrorDesc(err)))
	}

	if out == nil {
		return errResponse(codes.Internal, 0, "service returned nil message")
	}
	return respondMessage(out.(proto.Message), format)
}
