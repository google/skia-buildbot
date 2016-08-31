// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package grpcutil

import (
	"github.com/luci/luci-go/common/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	// Errf falls through to grpc.Errorf, with the notable exception that it isn't
	// named "Errorf" and, consequently, won't trigger "go vet" misuse errors.
	Errf = grpc.Errorf

	// OK is an empty grpc.OK status error.
	OK = Errf(codes.OK, "")

	// Canceled is an empty grpc.Canceled error.
	Canceled = Errf(codes.Canceled, "")

	// Unknown is an empty grpc.Unknown error.
	Unknown = Errf(codes.Unknown, "")

	// InvalidArgument is an empty grpc.InvalidArgument error.
	InvalidArgument = Errf(codes.InvalidArgument, "")

	// DeadlineExceeded is an empty grpc.DeadlineExceeded error.
	DeadlineExceeded = Errf(codes.DeadlineExceeded, "")

	// NotFound is an empty grpc.NotFound error.
	NotFound = Errf(codes.NotFound, "")

	// AlreadyExists is an empty grpc.AlreadyExists error.
	AlreadyExists = Errf(codes.AlreadyExists, "")

	// PermissionDenied is an empty grpc.PermissionDenied error.
	PermissionDenied = Errf(codes.PermissionDenied, "")

	// Unauthenticated is an empty grpc.Unauthenticated error.
	Unauthenticated = Errf(codes.Unauthenticated, "")

	// ResourceExhausted is an empty grpc.ResourceExhausted error.
	ResourceExhausted = Errf(codes.ResourceExhausted, "")

	// FailedPrecondition is an empty grpc.FailedPrecondition error.
	FailedPrecondition = Errf(codes.FailedPrecondition, "")

	// Aborted is an empty grpc.Aborted error.
	Aborted = Errf(codes.Aborted, "")

	// OutOfRange is an empty grpc.OutOfRange error.
	OutOfRange = Errf(codes.OutOfRange, "")

	// Unimplemented is an empty grpc.Unimplemented error.
	Unimplemented = Errf(codes.Unimplemented, "")

	// Internal is an empty grpc.Internal error.
	Internal = Errf(codes.Internal, "")

	// Unavailable is an empty grpc.Unavailable error.
	Unavailable = Errf(codes.Unavailable, "")

	// DataLoss is an empty grpc.DataLoss error.
	DataLoss = Errf(codes.DataLoss, "")
)

// WrapIfTransient wraps the supplied gRPC error with a transient wrapper if
// it has a transient gRPC code, as determined by IsTransientCode.
//
// If the supplied error is nil, nil will be returned.
//
// Note that non-gRPC errors will have code grpc.Unknown, which is considered
// transient, and be wrapped. This function should only be used on gRPC errors.
func WrapIfTransient(err error) error {
	if err == nil {
		return nil
	}

	if IsTransientCode(Code(err)) {
		err = errors.WrapTransient(err)
	}
	return err
}

const grpcCodeKey = "__grpcutil.Code"

// Code returns the gRPC code for a given error.
//
// In addition to the functionality of grpc.Code, this will unwrap any wrapped
// errors before asking for its code.
func Code(err error) codes.Code {
	if code := errors.ExtractData(err, grpcCodeKey); code != nil {
		return code.(codes.Code)
	}
	return grpc.Code(errors.Unwrap(err))
}

// Annotate begins annotating the error, and adds the given gRPC code.
// This code may be extracted with the Code function in this package.
func Annotate(err error, code codes.Code) *errors.Annotator {
	return errors.Annotate(err).D(grpcCodeKey, code)
}

// ToGRPCErr is a shorthand for Errf(Code(err), "%s", err)
func ToGRPCErr(err error) error {
	return Errf(Code(err), "%s", err)
}

// IsTransientCode returns true if a given gRPC code is associated with a
// transient gRPC error type.
func IsTransientCode(code codes.Code) bool {
	switch code {
	case codes.Internal, codes.Unknown, codes.Unavailable:
		return true

	default:
		return false
	}
}
