// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package assertions

import (
	"fmt"

	"github.com/smartystreets/goconvey/convey"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// ShouldHaveRPCCode is a goconvey assertion, asserting that the supplied
// "actual" value has a gRPC code value and, optionally, errors like a supplied
// message string.
//
// If no "expected" arguments are supplied, ShouldHaveRPCCode will assert that
// the result is codes.OK.
//
// The first "expected" argument, if supplied, is the gRPC codes.Code to assert.
//
// A second "expected" string may be optionally included. If included, the
// gRPC error message is asserted to contain the expected string using
// convey.ShouldContainSubstring.
func ShouldHaveRPCCode(actual interface{}, expected ...interface{}) string {
	aerr, ok := actual.(error)
	if !(ok || actual == nil) {
		return "actual argument must be an error."
	}

	var (
		ecode   codes.Code
		errLike string
	)
	switch len(expected) {
	case 2:
		var ok bool
		if errLike, ok = expected[1].(string); !ok {
			return fmt.Sprintf("The expected error substring must be a string, not a %T", expected[1])
		}
		fallthrough

	case 1:
		var ok bool
		if ecode, ok = expected[0].(codes.Code); !ok {
			return fmt.Sprintf("The code must be a codes.Code, not a %T", expected[0])
		}

	case 0:
		ecode = codes.OK

	default:
		return "Expected argument must have the form: [codes.Code[string]]"
	}

	if acode := grpc.Code(aerr); acode != ecode {
		return fmt.Sprintf("expected gRPC code %q (%d), not %q (%d), type %T: %v",
			ecode, ecode, acode, acode, actual, actual)
	}

	if errLike != "" {
		return convey.ShouldContainSubstring(grpc.ErrorDesc(aerr), errLike)
	}
	return ""
}

// ShouldBeRPCOK asserts that "actual" is an error that has a gRPC code value
// of codes.OK.
//
// Note that "nil" has an codes.OK value.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCOK(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.OK, expected)...)
}

// ShouldBeRPCInvalidArgument asserts that "actual" is an error that has a gRPC
// code value of codes.InvalidArgument.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCInvalidArgument(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.InvalidArgument, expected)...)
}

// ShouldBeRPCInternal asserts that "actual" is an error that has a gRPC code
// value of codes.Internal.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCInternal(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.Internal, expected)...)
}

// ShouldBeRPCUnknown asserts that "actual" is an error that has a gRPC code
// value of codes.Unknown.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCUnknown(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.Unknown, expected)...)
}

// ShouldBeRPCNotFound asserts that "actual" is an error that has a gRPC code
// value of codes.NotFound.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCNotFound(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.NotFound, expected)...)
}

// ShouldBeRPCPermissionDenied asserts that "actual" is an error that has a gRPC
// code value of codes.PermissionDenied.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCPermissionDenied(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.PermissionDenied, expected)...)
}

// ShouldBeRPCAlreadyExists asserts that "actual" is an error that has a gRPC
// code value of codes.AlreadyExists.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCAlreadyExists(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.AlreadyExists, expected)...)
}

// ShouldBeRPCUnauthenticated asserts that "actual" is an error that has a gRPC
// code value of codes.Unauthenticated.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCUnauthenticated(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.Unauthenticated, expected)...)
}

// ShouldBeRPCFailedPrecondition asserts that "actual" is an error that has a gRPC
// code value of codes.FailedPrecondition.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCFailedPrecondition(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.FailedPrecondition, expected)...)
}

// ShouldBeRPCAborted asserts that "actual" is an error that has a gRPC
// code value of codes.Aborted.
//
// One additional "expected" string may be optionally included. If included, the
// gRPC error's message is asserted to contain the expected string.
func ShouldBeRPCAborted(actual interface{}, expected ...interface{}) string {
	return ShouldHaveRPCCode(actual, prepend(codes.Aborted, expected)...)
}

func prepend(c codes.Code, exp []interface{}) []interface{} {
	args := make([]interface{}, len(exp)+1)
	args[0] = c
	copy(args[1:], exp)
	return args
}
