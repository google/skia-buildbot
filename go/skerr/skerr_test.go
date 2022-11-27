package skerr_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/skerr/alpha_test"
	"go.skia.org/infra/go/skerr/beta_test"
)

func TestCallStack(t *testing.T) {
	var stack []skerr.StackTrace
	callback := func() error {
		stack = skerr.CallStack(7, 0)
		return nil
	}
	var alpha alpha_test.Alpha
	alpha.SetWrappedCallback(callback)
	require.NoError(t, beta_test.CallAlpha(&alpha))
	require.Len(t, stack, 7)
	// Only assert line numbers in alpha_test.go and beta_test.go to avoid making the test brittle.
	require.Equal(t, "skerr.go", stack[0].File)        // CallStack -> runtime.Caller
	require.Equal(t, "skerr_test.go", stack[1].File)   // anonymous function in TestCallStack
	require.Equal(t, "alpha.go:14", stack[2].String()) // anonymous function in SetWrappedCallback
	require.Equal(t, "alpha.go:23", stack[3].String()) // Alpha.Call
	require.Equal(t, "beta.go:18", stack[4].String())  // callAlphaInternal
	require.Equal(t, "beta.go:22", stack[5].String())  // CallAlpha
	require.Equal(t, "skerr_test.go", stack[6].File)   // TestCallStack
}

func TestWrap(t *testing.T) {
	var alpha alpha_test.Alpha
	alpha.SetWrappedCallback(beta_test.GetGenericError)
	err := alpha.Call()
	require.Equal(t, beta_test.GenericError, skerr.Unwrap(err))
	require.Regexp(t, beta_test.GenericError.Error()+`\. At alpha\.go:15 alpha\.go:23 skerr_test\.go:\d+.*`, err.Error())
}

func TestUnwrapOtherErr(t *testing.T) {
	err := beta_test.GenericError
	require.Equal(t, err, skerr.Unwrap(err))
}

func TestFmt(t *testing.T) {
	const fmtStr = "Dog too small; dog is %d kg; minimum is %d kg."
	callback := func() error {
		return skerr.Fmt(fmtStr, 45, 50)
	}
	var alpha alpha_test.Alpha
	alpha.SetWrappedCallback(callback)
	err := alpha.Call()
	errStr := fmt.Sprintf(fmtStr, 45, 50)
	require.Equal(t, errStr, skerr.Unwrap(err).Error())
	require.Regexp(t, errStr+`\. At skerr_test\.go:\d+ alpha\.go:14 alpha\.go:23 skerr_test\.go:\d+.*`, err.Error())
}

func TestWrapfCreate(t *testing.T) {
	err := beta_test.Context1(beta_test.GetGenericError)
	require.Equal(t, beta_test.GenericError, skerr.Unwrap(err))
	require.Regexp(t, `When searching for 35 trees: human detected\. At beta.go:31 skerr_test\.go:\d+.*`, err.Error())
}

func TestWrapfAppend(t *testing.T) {
	callback := func() error {
		return skerr.Fmt("Dog lost interest")
	}
	err := beta_test.Context2(callback)
	require.Regexp(t, `When walking the dog: When searching for 35 trees: Dog lost interest\. At skerr_test\.go:\d+ beta.go:30 beta.go:38 skerr_test\.go:\d+.*`, err.Error())
}

func TestErrorWithContextUnwrap_ErrorIsWrapped_UnwrapReturnsNextErrorInTheChain(t *testing.T) {
	wrappedEOF := skerr.Wrap(io.EOF)
	require.Equal(t, errors.Unwrap(wrappedEOF), io.EOF)
}

func TestErrorWithContextUnwrap_ErrorIsWrapped_IsFindsCorrectErrorInTheChain(t *testing.T) {
	wrappedEOF := skerr.Wrap(io.EOF)
	require.True(t, errors.Is(wrappedEOF, io.EOF))
}

func TestErrorWithContextUnwrap_ErrorIsWrapped_AsExtractsCorrectErrorInTheChain(t *testing.T) {
	// Create an error that's interesting to unwrap.
	err := &json.SyntaxError{
		Offset: 32,
	}
	wrappedEOF := skerr.Wrapf(err, "decode JSON")

	// Use As to extract the Syntax error.
	var syntaxError *json.SyntaxError
	require.True(t, errors.As(wrappedEOF, &syntaxError))
	require.Equal(t, int64(32), syntaxError.Offset)
}
