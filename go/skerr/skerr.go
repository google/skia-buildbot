// skerr provides functions related to error reporting and stack traces.

package skerr

import (
	"fmt"
	"runtime"
	"strings"
)

// StackTrace identifies a filename (base filename only) and line number.
type StackTrace struct {
	File string
	Line int
}

func (st *StackTrace) String() string {
	return fmt.Sprintf("%s:%d", st.File, st.Line)
}

// CallStack returns a slice of StackTrace representing the current stack trace.
// The lines returned start at the depth specified by startAt: 0 means the call to CallStack,
// 1 means CallStack's caller, 2 means CallStack's caller's caller and so on. height means how
// many lines to include, counting deeper into the stack, with zero meaning to include all stack
// frames. If height is non-zero and there aren't enough stack frames, a dummy value is used
// instead.
//
// Suppose the stacktrace looks like:
// skerr.go:300  <- the call to runtime.Caller in skerr.CallStack
// alpha.go:123
// beta.go:456
// gamma.go:789
// delta.go:123
// main.go: 70
// A typical call may look like skerr.CallStack(6, 1), which returns
// [{File:alpha.go, Line:123}, {File:beta.go, Line:456},...,
//  {File:main.go, Line:70}, {File:???, Line:1}], omitting the not-helpful reference to
// CallStack and padding the response with a dummy value, since the stack was not tall enough to
// show 6 items, starting at the second one.
func CallStack(height, startAt int) []StackTrace {
	stack := []StackTrace{}
	for i := 0; ; i++ {
		_, file, line, ok := runtime.Caller(startAt + i)
		if !ok {
			if height <= 0 {
				break
			}
			file = "???"
			line = 1
		} else {
			slash := strings.LastIndex(file, "/")
			if slash >= 0 {
				file = file[slash+1:]
			}
		}
		stack = append(stack, StackTrace{File: file, Line: line})
		if height > 0 && len(stack) >= height {
			break
		}
	}
	return stack
}

// ErrorWithContext contains an original error with context info and a stack trace. It implements
// the error interface.
type ErrorWithContext struct {
	// Wrapped is the original error. Never nil.
	Wrapped error
	// CallStack captures the caller's stack at the time the ErrorWithContext was created; most recent
	// call first.
	CallStack []StackTrace
	// Context contains additional info from calls to Wrapf. The slice is in order of calls to Wrapf,
	// which Error() prints in reverse.
	Context []string
}

func (err *ErrorWithContext) Error() string {
	var out strings.Builder
	for i := len(err.Context) - 1; i >= 0; i-- {
		_, _ = out.WriteString(err.Context[i])
		_, _ = out.WriteString(": ")
	}
	_, _ = out.WriteString(err.Wrapped.Error())
	if len(err.CallStack) > 0 {
		_, _ = out.WriteString(". At")
		for _, st := range err.CallStack {
			_, _ = fmt.Fprintf(&out, " %s:%d", st.File, st.Line)
		}
	}
	return out.String()
}

// Unwrap allows unwrapping an error as implemented in the `errors` standard
// library.
func (err *ErrorWithContext) Unwrap() error {
	return err.Wrapped
}

func tryCast(err error) (*ErrorWithContext, bool) {
	if wrapper, ok := err.(*ErrorWithContext); ok {
		return wrapper, true
	} else {
		return nil, false
	}
}

func wrap(err error, startAt int) error {
	if _, ok := tryCast(err); ok {
		return err
	}
	return &ErrorWithContext{
		Wrapped:   err,
		CallStack: CallStack(0, startAt),
	}
}

// Wrap adds stack trace info to err, if not already present. The return value will be of type
// ErrorWithContext. If err is nil, nil is returned instead.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	return wrap(err, 3)
}

// Unwrap returns the original error if err is ErrWithContext, otherwise just returns err.
func Unwrap(err error) error {
	if wrapper, ok := tryCast(err); ok {
		return wrapper.Wrapped
	}
	return err
}

// Fmt is equivalent to Wrap(fmt.Errorf(...)).
func Fmt(fmtStr string, args ...interface{}) error {
	return wrap(fmt.Errorf(fmtStr, args...), 3)
}

// Wrapf adds context and stack trace info to err. Existing stack trace info will be preserved. The
// return value will be of type ErrorWithContext.
// Example: sklog.Wrapf(err, "When loading %d items from %s", count, url)
//  If err is nil, nil is returned instead.
func Wrapf(err error, fmtStr string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	newContext := fmt.Sprintf(fmtStr, args...)
	if wrapper, ok := tryCast(err); ok {
		callStack := wrapper.CallStack
		if len(callStack) == 0 {
			callStack = CallStack(0, 2)
		}
		return &ErrorWithContext{
			Wrapped:   wrapper.Wrapped,
			CallStack: callStack,
			Context:   append(wrapper.Context, newContext),
		}
	} else {
		return &ErrorWithContext{
			Wrapped:   err,
			CallStack: CallStack(0, 2),
			Context:   []string{newContext},
		}
	}
}
