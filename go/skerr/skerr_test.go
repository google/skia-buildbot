package skerr_test

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/skerr/alpha_test"
	"go.skia.org/infra/go/skerr/beta_test"
	"go.skia.org/infra/go/testutils"
)

func TestCallStack(t *testing.T) {
	testutils.SmallTest(t)
	var stack []skerr.StackTrace
	callback := func() error {
		stack = skerr.CallStack(7, 0)
		return nil
	}
	var alpha alpha_test.Alpha
	alpha.SetWrappedCallback(callback)
	assert.NoError(t, beta_test.CallAlpha(&alpha))
	assert.Len(t, stack, 7)
	// Only assert line numbers in alpha_test.go and beta_test.go to avoid making the test brittle.
	assert.Equal(t, "skerr.go", stack[0].File)        // CallStack -> runtime.Caller
	assert.Equal(t, "skerr_test.go", stack[1].File)   // anonymous function in TestCallStack
	assert.Equal(t, "alpha.go:14", stack[2].String()) // anonymous function in SetWrappedCallback
	assert.Equal(t, "alpha.go:23", stack[3].String()) // Alpha.Call
	assert.Equal(t, "beta.go:18", stack[4].String())  // callAlphaInternal
	assert.Equal(t, "beta.go:22", stack[5].String())  // CallAlpha
	assert.Equal(t, "skerr_test.go", stack[6].File)   // TestCallStack
}

func TestWrap(t *testing.T) {
	testutils.SmallTest(t)
	var alpha alpha_test.Alpha
	alpha.SetWrappedCallback(beta_test.GetGenericError)
	err := alpha.Call()
	assert.Equal(t, beta_test.GenericError, skerr.Unwrap(err))
	assert.Regexp(t, beta_test.GenericError.Error()+`\. At alpha\.go:15 alpha\.go:23 skerr_test\.go:\d+.*`, err.Error())
}

func TestUnwrapOtherErr(t *testing.T) {
	testutils.SmallTest(t)
	err := beta_test.GenericError
	assert.Equal(t, err, skerr.Unwrap(err))
}

func TestFmt(t *testing.T) {
	testutils.SmallTest(t)
	const fmtStr = "Dog too small; dog is %d kg; minimum is %d kg."
	callback := func() error {
		return skerr.Fmt(fmtStr, 45, 50)
	}
	var alpha alpha_test.Alpha
	alpha.SetWrappedCallback(callback)
	err := alpha.Call()
	errStr := fmt.Sprintf(fmtStr, 45, 50)
	assert.Equal(t, errStr, skerr.Unwrap(err).Error())
	assert.Regexp(t, errStr+`\. At skerr_test\.go:\d+ alpha\.go:14 alpha\.go:23 skerr_test\.go:\d+.*`, err.Error())
}

func TestWrapfCreate(t *testing.T) {
	testutils.SmallTest(t)
	err := beta_test.Context1(beta_test.GetGenericError)
	assert.Equal(t, beta_test.GenericError, skerr.Unwrap(err))
	assert.Regexp(t, `When searching for 35 trees: human detected\. At beta.go:31 skerr_test\.go:\d+.*`, err.Error())
}

func TestWrapfAppend(t *testing.T) {
	testutils.SmallTest(t)
	callback := func() error {
		return skerr.Fmt("Dog lost interest")
	}
	err := beta_test.Context2(callback)
	assert.Regexp(t, `When walking the dog: When searching for 35 trees: Dog lost interest\. At skerr_test\.go:\d+ beta.go:30 beta.go:38 skerr_test\.go:\d+.*`, err.Error())
}
