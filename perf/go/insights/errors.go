// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// Code is a canonical error code that is safe to return to API clients. The set of codes follows
// contract C1 (error envelope) of the perf.v1 API plan.
type Code string

const (
	CodeOK                Code = "OK"
	CodeInvalidArgument   Code = "INVALID_ARGUMENT"
	CodeNotFound          Code = "NOT_FOUND"
	CodeUnauthenticated   Code = "UNAUTHENTICATED"
	CodePermissionDenied  Code = "PERMISSION_DENIED"
	CodeResourceExhausted Code = "RESOURCE_EXHAUSTED"
	CodeDeadlineExceeded  Code = "DEADLINE_EXCEEDED"
	CodeInternal          Code = "INTERNAL"
)

// HTTPStatus returns the HTTP status code that corresponds to the Code.
func (c Code) HTTPStatus() int {
	switch c {
	case CodeOK:
		return http.StatusOK
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodePermissionDenied:
		return http.StatusForbidden
	case CodeResourceExhausted:
		return http.StatusTooManyRequests
	case CodeDeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// Error is an error whose Message and Hint are safe to show to API clients, including LLM
// agents. Hint tells an agent what to do next, e.g. which tool returns valid input.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Hint == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s (hint: %s)", e.Code, e.Message, e.Hint)
}

// ErrorEnvelope is the JSON body of every failed REST response (contract C1).
type ErrorEnvelope struct {
	Error *Error `json:"error"`
}

func newError(code Code, hint string, format string, args ...interface{}) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Hint: hint}
}

// InvalidArgument returns an INVALID_ARGUMENT error.
func InvalidArgument(hint string, format string, args ...interface{}) *Error {
	return newError(CodeInvalidArgument, hint, format, args...)
}

// NotFound returns a NOT_FOUND error.
func NotFound(hint string, format string, args ...interface{}) *Error {
	return newError(CodeNotFound, hint, format, args...)
}

// ToPublicError converts any error into an *Error that is safe to return to clients.
//
// SECURITY: errors that are not already an *Error become a generic INTERNAL error, because store
// errors embed SQL statements and other details that must not leak to clients. Callers are
// responsible for logging the original error.
func ToPublicError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &Error{
			Code:    CodeDeadlineExceeded,
			Message: "the request took too long",
			Hint:    "narrow the request (fewer days, a smaller page_size or fewer ids) and retry",
		}
	}
	return &Error{Code: CodeInternal, Message: "internal error"}
}
