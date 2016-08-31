// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package errors

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/data/text/indented"
	"github.com/luci/luci-go/common/iotools"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/runtime/goroutine"
)

// Datum is a single data entry value for StackContext.Data.
type Datum struct {
	Value       interface{}
	StackFormat string
}

// Data is used to add data to a StackContext.
type Data map[string]Datum

type stack struct {
	id     goroutine.ID
	frames []uintptr
}

func (s *stack) findPointOfDivergence(other *stack) int {
	// TODO(iannucci): can we optimize this search routine to not overly penalize
	// tail-recursive functions? Searching 'up' from both stacks doesn't work in
	// the face of recursion because there will be multiple ambiguous stack
	// frames. The algorithm below is correct, but it potentially collects and
	// then throws away a bunch of data (e.g. if we could walk up from the current
	// stack and then find the intersection in the captured stack, we could stop
	// walking at that point instead of walking all the way back from the root on
	// every Annotate call.
	if s.id != other.id {
		panic(fmt.Errorf(
			"finding intersection between unrelated stacks: %d v %d", s.id, other.id))
	}

	myIdx := len(s.frames) - 1
	oIdx := len(other.frames) - 1

	for s.frames[myIdx] == other.frames[oIdx] {
		myIdx--
		oIdx--
	}

	return myIdx
}

// StackContexter is the interface that an error may implement if it has data
// associated with a specific stack frame.
type StackContexter interface {
	StackContext() StackContext
}

// StackFrameInfo holds a stack and an index into that stack for association
// with StackContexts.
type StackFrameInfo struct {
	frameIdx int
	forStack *stack
}

// StackContext represents the annotation data associated with an error, or an
// annotation of an error.
type StackContext struct {
	FrameInfo StackFrameInfo
	// Reason is the publicly-facing reason, and will show up in the Error()
	// string.
	Reason string

	// InternalReason is used for printing tracebacks, but is otherwise formatted
	// like Reason.
	InternalReason string
	Data           Data

	Transient bool
}

// We're looking for %(sometext) which is not preceded by a %. sometext may be
// any characters except for a close paren.
//
// Submatch indices:
// [0:1] Full match
// [2:3] Text before the (...) pair (including the '%').
// [4:5] (key)
var namedFormatMatcher = regexp.MustCompile(`((?:^|[^%])%)\(([^)]+)\)`)

// Format uses the data contained in this Data map to format the provided
// string. Items from the map are looked up in python dict-format style, e.g.
//
//   %(key)d
//
// Will look up the item "key", and then format as a decimal number it using the
// value for "key" in this Data map. Like python, a given item may appear
// multiple times in the format string.
//
// All formatting directives are identical to the ones used by fmt.Sprintf.
func (d Data) Format(format string) string {
	smi := namedFormatMatcher.FindAllStringSubmatchIndex(format, -1)

	var (
		parts = make([]string, 0, len(smi)+1)
		args  = make([]interface{}, 0, len(smi))
		pos   = 0
	)
	for _, match := range smi {
		// %(key)s => %s
		parts = append(parts, format[pos:match[3]])
		pos = match[1]

		// Add key to args.
		key := format[match[4]:match[5]]
		if v, ok := d[key]; ok {
			args = append(args, v.Value)
		} else {
			args = append(args, fmt.Sprintf("MISSING(key=%q)", key))
		}
	}
	parts = append(parts, format[pos:])
	return fmt.Sprintf(strings.Join(parts, ""), args...)
}

// RenderPublic renders the public error.Error()-style string for this frame,
// using the Reason and Data to produce a human readable string.
func (s *StackContext) RenderPublic(inner error) string {
	if s.Reason == "" {
		if inner != nil {
			return inner.Error()
		}
		return ""
	}

	basis := s.Data.Format(s.Reason)
	if inner != nil {
		return fmt.Sprintf("%s: %s", basis, inner)
	}
	return basis
}

// render renders the frame as a single entry in a stack trace. This looks like:
//
//  I am an internal reson formatted with key1: value
//  reason: "The literal content of the Reason field: %(key2)d"
//  "key1" = "value"
//  "key2" = 10
func (s *StackContext) render() Lines {
	siz := len(s.Data)
	if s.InternalReason != "" {
		siz++
	}
	if s.Reason != "" {
		siz++
	}
	if siz == 0 {
		return nil
	}

	ret := make(Lines, 0, siz)

	if s.InternalReason != "" {
		ret = append(ret, s.Data.Format(s.InternalReason))
	}
	if s.Reason != "" {
		ret = append(ret, fmt.Sprintf("reason: %q", s.Data.Format(s.Reason)))
	}
	if s.Transient {
		ret = append(ret, "transient: true")
	}

	if len(s.Data) > 0 {
		for k, v := range s.Data {
			if v.StackFormat == "" || v.StackFormat == "%#v" {
				ret = append(ret, fmt.Sprintf("%q = %#v", k, v.Value))
			} else {
				ret = append(ret, fmt.Sprintf("%q = "+v.StackFormat, k, v.Value))
			}
		}
		sort.Strings(ret[len(ret)-len(s.Data):])
	}

	return ret
}

// AddData does a 'dict.update' addition of the data.
func (s *StackContext) AddData(data Data) {
	if s.Data == nil {
		s.Data = make(Data, len(data))
	}
	for k, v := range data {
		s.Data[k] = v
	}
}

// AddDatum adds a single data item to the Data in this frame
func (s *StackContext) AddDatum(key string, value interface{}, format string) {
	if s.Data == nil {
		s.Data = Data{key: {value, format}}
	} else {
		s.Data[key] = Datum{value, format}
	}
}

type terminalStackError struct {
	error
	finfo StackFrameInfo
}

var _ interface {
	error
	StackContexter
} = (*terminalStackError)(nil)

func (e *terminalStackError) StackContext() StackContext { return StackContext{FrameInfo: e.finfo} }

type annotatedError struct {
	inner error
	ctx   StackContext
}

var _ interface {
	error
	StackContexter
	Wrapped
} = (*annotatedError)(nil)

func (e *annotatedError) Error() string              { return e.ctx.RenderPublic(e.inner) }
func (e *annotatedError) StackContext() StackContext { return e.ctx }
func (e *annotatedError) InnerError() error          { return e.inner }
func (e *annotatedError) IsTransient() bool          { return e.ctx.Transient }

// Annotator is a builder for annotating errors. Obtain one by calling Annotate
// on an existing error or using Reason.
//
// See the example test for Annotate to see how this is meant to be used.
type Annotator struct {
	inner error
	ctx   StackContext
}

// Reason adds a PUBLICLY READABLE reason string (for humans) to this error.
//
// You should assume that end-users (including unauthenticated end users) may
// see the text in here.
//
// These reasons will be used to compose the result of the final Error() when
// rendering this error, and will also be used to decorate the error
// annotation stack when logging the error using the Log function.
//
// In a webserver context, if you don't want users to see some information about
// this error, don't put it in the Reason.
//
// This explanation may have formatting instructions in the form of:
//   %(key)...
// where key is the name of one of the entries submitted to either D or Data.
// The `...` may be any Printf-compatible formatting directive.
func (a *Annotator) Reason(reason string) *Annotator {
	if a == nil {
		return a
	}
	a.ctx.Reason = reason
	return a
}

// InternalReason adds a stack-trace-only internal reason string (for humans) to
// this error. This is formatted like Reason, but will not be visible in the
// Error() string.
func (a *Annotator) InternalReason(reason string) *Annotator {
	if a == nil {
		return a
	}
	a.ctx.InternalReason = reason
	return a
}

// D adds a single datum to this error. Only one format may be specified. If
// format is omitted or the empty string, the format "%#v" will be used.
func (a *Annotator) D(key string, value interface{}, format ...string) *Annotator {
	if a == nil {
		return a
	}
	formatVal := ""
	switch len(format) {
	case 0:
	case 1:
		formatVal = format[0]
	default:
		panic(fmt.Errorf("len(format) > 1: %d", len(format)))
	}
	a.ctx.AddDatum(key, value, formatVal)
	return a
}

// Data adds data to this error.
func (a *Annotator) Data(data Data) *Annotator {
	if a == nil {
		return a
	}
	a.ctx.AddData(data)
	return a
}

// Transient marks this error as transient. If the inner error is already
// transient, this has no effect.
func (a *Annotator) Transient() *Annotator {
	if a == nil {
		return a
	}
	if !IsTransient(a.inner) {
		a.ctx.Transient = true
	}
	return a
}

// Err returns the finalized annotated error.
func (a *Annotator) Err() error {
	if a == nil {
		return nil
	}
	return (*annotatedError)(a)
}

// Log logs the full error. If this is an Annotated error, it will log the full
// stack information as well.
func Log(c context.Context, err error) {
	log := logging.Get(c)
	for _, l := range RenderStack(err).ToLines() {
		log.Errorf("%s", l)
	}
}

// Lines is just a list of printable lines.
type Lines []string

// RenderedFrame represents a single, rendered stack frame.
type RenderedFrame struct {
	Pkg      string
	File     string
	LineNum  int
	FuncName string

	// Wrappers is any frame-info-less errors.Wrapped that were encountered when
	// rendering that didn't have any associated frame info: this is the closest
	// frame to where they were added to the error.
	Wrappers []Lines

	// Annotations is any Annotate context associated directly with this Frame.
	Annotations []Lines
}

var nlSlice = []byte{'\n'}

// DumpWrappersTo formats the Wrappers portion of this RenderedFrame.
func (r *RenderedFrame) DumpWrappersTo(w io.Writer, from, to int) (n int, err error) {
	return iotools.WriteTracker(w, func(rawWriter io.Writer) error {
		w := &indented.Writer{Writer: rawWriter, UseSpaces: true}
		fmt.Fprintf(w, "From frame %d to %d, the following wrappers were found:\n", from, to)
		for i, wrp := range r.Wrappers {
			if i != 0 {
				w.Write(nlSlice)
			}
			w.Level = 2
			for i, line := range wrp {
				if i == 0 {
					fmt.Fprintf(w, "%s\n", line)
					w.Level += 2
				} else {
					fmt.Fprintf(w, "%s\n", line)
				}
			}
		}
		return nil
	})
}

// DumpTo formats the Header and Annotations for this RenderedFrame.
func (r *RenderedFrame) DumpTo(w io.Writer, idx int) (n int, err error) {
	return iotools.WriteTracker(w, func(rawWriter io.Writer) error {
		w := &indented.Writer{Writer: rawWriter, UseSpaces: true}

		fmt.Fprintf(w, "#%d %s/%s:%d - %s()\n", idx, r.Pkg, r.File,
			r.LineNum, r.FuncName)
		w.Level += 2
		switch len(r.Annotations) {
		case 0:
			// pass
		case 1:
			for _, line := range r.Annotations[0] {
				fmt.Fprintf(w, "%s\n", line)
			}
		default:
			for i, ann := range r.Annotations {
				fmt.Fprintf(w, "annotation #%d:\n", i)
				w.Level += 2
				for _, line := range ann {
					fmt.Fprintf(w, "%s\n", line)
				}
				w.Level -= 2
			}
		}
		return nil
	})
}

// RenderedStack is a single rendered stack from one goroutine.
type RenderedStack struct {
	GoID   goroutine.ID
	Frames []*RenderedFrame
}

// DumpTo formats the full stack.
func (r *RenderedStack) DumpTo(w io.Writer, excludePkgs ...string) (n int, err error) {
	excludeSet := stringset.NewFromSlice(excludePkgs...)

	return iotools.WriteTracker(w, func(w io.Writer) error {
		fmt.Fprintf(w, "goroutine %d:\n", r.GoID)

		lastIdx := 0
		needNL := false
		skipCount := 0
		skipPkg := ""
		flushSkips := func(extra string) {
			if skipCount != 0 {
				if needNL {
					w.Write(nlSlice)
					needNL = false
				}
				fmt.Fprintf(w, "... skipped %d frames in pkg %q...\n%s", skipCount, skipPkg, extra)
				skipCount = 0
				skipPkg = ""
			}
		}
		for i, f := range r.Frames {
			if needNL {
				w.Write(nlSlice)
				needNL = false
			}
			if excludeSet.Has(f.Pkg) {
				if skipPkg == f.Pkg {
					skipCount++
				} else {
					flushSkips("")
					skipCount++
					skipPkg = f.Pkg
				}
				continue
			}
			flushSkips("\n")
			if len(f.Wrappers) > 0 {
				f.DumpWrappersTo(w, lastIdx, i)
				w.Write(nlSlice)
			}
			if len(f.Annotations) > 0 {
				lastIdx = i
				needNL = true
			}
			f.DumpTo(w, i)
		}
		flushSkips("")

		return nil
	})
}

// RenderedError is a series of RenderedStacks, one for each goroutine that the
// error was annotated on.
type RenderedError struct {
	OriginalError string
	Stacks        []*RenderedStack
}

// ToLines renders a full-information stack trace as a series of lines.
func (r *RenderedError) ToLines(excludePkgs ...string) Lines {
	buf := bytes.Buffer{}
	r.DumpTo(&buf, excludePkgs...)
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

// DumpTo writes the full-information stack trace to the writer.
func (r *RenderedError) DumpTo(w io.Writer, excludePkgs ...string) (n int, err error) {
	return iotools.WriteTracker(w, func(w io.Writer) error {
		if r.OriginalError != "" {
			fmt.Fprintf(w, "original error: %s\n\n", r.OriginalError)
		}

		for i := len(r.Stacks) - 1; i >= 0; i-- {
			if i != len(r.Stacks)-1 {
				w.Write(nlSlice)
			}
			r.Stacks[i].DumpTo(w, excludePkgs...)
		}
		return nil
	})
}

func frameHeaderDetails(frm uintptr) (pkg, filename, funcname string, lineno int) {
	// this `frm--` is to get the correct line/function information, since the
	// Frame is actually the `return` pc. See runtime.Callers.
	frm--

	fn := runtime.FuncForPC(frm)
	file, lineno := fn.FileLine(frm)

	var dirpath string
	dirpath, filename = filepath.Split(file)
	pkgTopLevelName := filepath.Base(dirpath)

	fnName := fn.Name()
	lastSlash := strings.LastIndex(fnName, "/")
	if lastSlash == -1 {
		funcname = fnName
		pkg = pkgTopLevelName
	} else {
		funcname = fnName[lastSlash+1:]
		pkg = fmt.Sprintf("%s/%s", fnName[:lastSlash], pkgTopLevelName)
	}
	return
}

// RenderStack renders the error to a RenderedError.
func RenderStack(err error) *RenderedError {
	ret := &RenderedError{}

	lastAnnotatedFrame := 0
	var wrappers = []Lines{}
	getCurFrame := func(fi *StackFrameInfo) *RenderedFrame {
		if len(ret.Stacks) == 0 || ret.Stacks[len(ret.Stacks)-1].GoID != fi.forStack.id {
			lastAnnotatedFrame = len(fi.forStack.frames) - 1
			toAdd := &RenderedStack{
				GoID:   fi.forStack.id,
				Frames: make([]*RenderedFrame, len(fi.forStack.frames)),
			}
			for i, frm := range fi.forStack.frames {
				pkgPath, filename, functionName, line := frameHeaderDetails(frm)
				toAdd.Frames[i] = &RenderedFrame{
					Pkg: pkgPath, File: filename, LineNum: line, FuncName: functionName}
			}
			ret.Stacks = append(ret.Stacks, toAdd)
		}
		curStack := ret.Stacks[len(ret.Stacks)-1]

		if fi.frameIdx < lastAnnotatedFrame {
			lastAnnotatedFrame = fi.frameIdx
			frm := curStack.Frames[lastAnnotatedFrame]
			frm.Wrappers = wrappers
			wrappers = nil
			return frm
		}
		return curStack.Frames[lastAnnotatedFrame]
	}

	for err != nil {
		if sc, ok := err.(StackContexter); ok {
			ctx := sc.StackContext()
			if stk := ctx.FrameInfo.forStack; stk != nil {
				frm := getCurFrame(&ctx.FrameInfo)
				if rendered := ctx.render(); len(rendered) > 0 {
					frm.Annotations = append(frm.Annotations, rendered)
				}
			} else {
				wrappers = append(wrappers, ctx.render())
			}
		} else {
			wrappers = append(wrappers, Lines{fmt.Sprintf("unknown wrapper %T", err)})
		}
		switch x := err.(type) {
		case MultiError:
			// TODO(riannucci): it's kinda dumb that we have to walk the MultiError
			// twice (once in its StackContext method, and again here).
			err = x.First()
		case Wrapped:
			err = x.InnerError()
		default:
			ret.OriginalError = err.Error()
			err = nil
		}
	}

	return ret
}

// Annotate captures the current stack frame and returns a new annotatable
// error. You can add additional metadata to this error with its methods and
// then get the new derived error with the Err() function.
//
// If this is passed nil, it will return a no-op Annotator whose .Err() function
// will also return nil.
//
// The original error may be recovered by using Wrapped.InnerError on the
// returned error.
//
// Rendering the derived error with Error() will render a summary version of all
// the Reasons as well as the initial underlying errors Error() text. It is
// intended that the initial underlying error and all annotated Reasons only
// contain user-visible information, so that the accumulated error may be
// returned to the user without worrying about leakage.
func Annotate(err error) *Annotator {
	if err == nil {
		return nil
	}
	return &Annotator{err, StackContext{FrameInfo: StackFrameInfoForError(1, err)}}
}

// Reason builds a new Annotator starting with reason. This allows you to use
// all the formatting directives you would normally use with Annotate, in case
// your originating error needs formatting directives:
//
//   errors.Reason("something bad: %(value)d").D("value", 100)).Err()
//
// Prefer this form to errors.New(fmt.Sprintf("..."))
func Reason(reason string) *Annotator {
	currentStack := captureStack(1)
	frameInfo := StackFrameInfo{0, currentStack}
	return (&Annotator{nil, StackContext{FrameInfo: frameInfo}}).Reason(reason)
}

// New is an API-compatible version of the standard errors.New function. Unlike
// the stdlib errors.New, this will capture the current stack information at the
// place this error was created.
func New(msg string) error {
	return &terminalStackError{errors.New(msg),
		StackFrameInfo{forStack: captureStack(1)}}
}

func captureStack(skip int) *stack {
	fullStk := stack{goroutine.CurID(), nil}
	stk := make([]uintptr, 16)
	offset := skip + 2
	for n := len(stk); n == len(stk); {
		n = runtime.Callers(offset, stk)
		offset += n
		fullStk.frames = append(fullStk.frames, stk[:n]...)
	}
	return &fullStk
}

func getCapturedStack(err error) (ret *stack) {
	Walk(err, func(err error) bool {
		if sc, ok := err.(StackContexter); ok {
			ret = sc.StackContext().FrameInfo.forStack
			return false
		}
		return true
	})
	return
}

// StackFrameInfoForError returns a StackFrameInfo suitable for use to implement
// the StackContexter interface.
//
// It skips the provided number of frames when collecting the current trace
// (which should be equal to the number of functions between your error library
// and the user's code).
//
// The returned StackFrameInfo will find the appropriate frame in the error's
// existing stack information (if the error was created with errors.New), or
// include the current stack if it was not.
func StackFrameInfoForError(skip int, err error) StackFrameInfo {
	currentStack := captureStack(skip + 1)
	currentlyCapturedStack := getCapturedStack(err)
	if currentlyCapturedStack == nil || currentStack.id != currentlyCapturedStack.id {
		// This is the very first annotation on this error OR
		// We switched goroutines.
		return StackFrameInfo{forStack: currentStack}
	}
	return StackFrameInfo{
		frameIdx: currentlyCapturedStack.findPointOfDivergence(currentStack),
		forStack: currentlyCapturedStack,
	}
}

// ExtractData walks the error and extracts the given key's data from
// Annotations (e.g. data added with D(key, <value>) will return <value>).
//
// The first matching key encountered (e.g. highest up the callstack) will be
// returned.
//
// If the error does not contain key at all, this returns nil.
func ExtractData(err error, key string) (ret interface{}) {
	Walk(err, func(err error) bool {
		if sc, ok := err.(StackContexter); ok {
			if d, ok := sc.StackContext().Data[key]; ok {
				ret = d.Value
				return false
			}
		}
		return true
	})
	return
}
