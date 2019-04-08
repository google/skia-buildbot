// Copyright (c) 2009 The Go Authors. All rights reserved.

// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:

//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.

// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Deep equality test via reflection

package deepequal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/davecgh/go-spew/spew"
	"github.com/pmezard/go-difflib/difflib"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

// During deepValueEqual, must keep track of checks that are
// in progress. The comparison algorithm assumes that all
// checks in progress are true when it reencounters them.
// Visited comparisons are stored in a map indexed by visit.
type visit struct {
	a1  unsafe.Pointer
	a2  unsafe.Pointer
	typ reflect.Type
}

// Tests for deep equality using reflected types. The map argument tracks
// comparisons that have already been seen, which allows short circuiting on
// recursive types.
func deepValueEqual(v1, v2 reflect.Value, visited map[visit]bool, depth int) bool {
	if !v1.IsValid() || !v2.IsValid() {
		return v1.IsValid() == v2.IsValid()
	}
	if v1.Type() != v2.Type() {
		return false
	}

	// We want to avoid putting more in the visited map than we need to.
	// For any possible reference cycle that might be encountered,
	// hard(t) needs to return true for at least one of the types in the cycle.
	hard := func(k reflect.Kind) bool {
		switch k {
		case reflect.Map, reflect.Slice, reflect.Ptr, reflect.Interface:
			return true
		}
		return false
	}

	if v1.CanAddr() && v2.CanAddr() && hard(v1.Kind()) {
		addr1 := unsafe.Pointer(v1.UnsafeAddr())
		addr2 := unsafe.Pointer(v2.UnsafeAddr())
		if uintptr(addr1) > uintptr(addr2) {
			// Canonicalize order to reduce number of entries in visited.
			// Assumes non-moving garbage collector.
			addr1, addr2 = addr2, addr1
		}

		// Short circuit if references are already seen.
		typ := v1.Type()
		v := visit{addr1, addr2, typ}
		if visited[v] {
			return true
		}

		// Remember for later.
		visited[v] = true
	}

	switch v1.Kind() {
	case reflect.Array:
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited, depth+1) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for i := 0; i < v1.Len(); i++ {
			if !deepValueEqual(v1.Index(i), v2.Index(i), visited, depth+1) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if v1.IsNil() || v2.IsNil() {
			return v1.IsNil() == v2.IsNil()
		}
		return deepValueEqual(v1.Elem(), v2.Elem(), visited, depth+1)
	case reflect.Ptr:
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		return deepValueEqual(v1.Elem(), v2.Elem(), visited, depth+1)
	case reflect.Struct:

		// https://stackoverflow.com/a/43918797 tells us that we can make an addressable
		// copy of the struct (if it isn't addressable already)
		// This will allow us to compare unexported fields later using unsafe.Pointer
		// Note, we shouldn't just blindly make a new, addressable copy of the element,
		// because that defeats the infinite recursion protection from above.
		v1copy := v1
		if !v1.CanAddr() {
			v1copy = reflect.New(v1.Type()).Elem()
			v1copy.Set(v1)
		}
		v2copy := v2
		if !v2.CanAddr() {
			v2copy = reflect.New(v2.Type()).Elem()
			v2copy.Set(v2)
		}

		// If Equal is defined on the struct, call it and use the result if valid.
		// This is especially helpful in Go 1.9, with the introduction of monotonic
		// times. reflect.DeepEqual() would compare the (unexported) monotonic field
		// and incorrectly deduce times as being not equal. By calling the Equal
		// method when available, we allow struct writers to control the DeepEqual
		// behavior.

		// Do PtrTo to expose all methods on the type and pointers of the type (the
		// former comes with the latter)
		tp := reflect.PtrTo(v1.Type())
		if equal, found := tp.MethodByName("Equal"); found {
			if equal.Type.NumIn() == 2 && // Two inputs (caller + object checking for equality)
				equal.Type.In(1).AssignableTo(v2copy.Type()) && // make sure v2copy can be passed in to the function in the second slot.
				equal.Type.NumOut() == 1 && // One output, which is exactly a bool
				equal.Type.Out(0).Kind() == reflect.Bool {
				// Actually call the function. Since we requested methods with a pointer reciever
				// we need to use Addr() on the caller object.
				retvals := equal.Func.Call([]reflect.Value{v1copy.Addr(), v2copy})
				if len(retvals) == 1 {
					if isEqual, ok := retvals[0].Interface().(bool); ok {
						return isEqual
					}
				}
				// If any of that failed, the Equal method we found isn't something we can use, so
				// fallthrough to the regular field by field comparison.
			}
		}
		for i, n := 0, v1.NumField(); i < n; i++ {
			// Then, we can use unsafe.Pointer to get access to the field, even if it is
			// unexported.
			f1 := v1copy.Field(i)
			if !f1.CanInterface() {
				f1 = reflect.NewAt(f1.Type(), unsafe.Pointer(f1.UnsafeAddr())).Elem()
			}
			f2 := v2copy.Field(i)
			if !f2.CanInterface() {
				f2 = reflect.NewAt(f2.Type(), unsafe.Pointer(f2.UnsafeAddr())).Elem()
			}
			if !deepValueEqual(f1, f2, visited, depth+1) {
				return false
			}
		}
		return true
	case reflect.Map:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for _, k := range v1.MapKeys() {
			val1 := v1.MapIndex(k)
			val2 := v2.MapIndex(k)
			if !val1.IsValid() || !val2.IsValid() || !deepValueEqual(v1.MapIndex(k), v2.MapIndex(k), visited, depth+1) {
				return false
			}
		}
		return true
	case reflect.Func:
		if v1.IsNil() && v2.IsNil() {
			return true
		}
		// Can't do better than this:
		return false
	default:
		// Normal equality suffices

		// This is different from the reflect.DeepEqual because we don't have access to
		// some unexported functions in the reflect package that let us get access
		// to unexported variables. Because of the unsafe.Pointer() logic done above
		// we can safely make a call to Interface(), which would typically fail for
		// unexported fields.
		return v1.Interface() == v2.Interface()
	}
}

// DeepEqual reports whether x and y are ``deeply equal,'' defined as follows.
// Two values of identical type are deeply equal if one of the following cases applies.
// Values of distinct types are never deeply equal.
//
// Array values are deeply equal when their corresponding elements are deeply equal.
//
// Struct values are deeply equal if their corresponding fields,
// both exported and unexported, are deeply equal.
//
// Func values are deeply equal if both are nil; otherwise they are not deeply equal.
//
// Interface values are deeply equal if they hold deeply equal concrete values.
//
// Map values are deeply equal when all of the following are true:
// they are both nil or both non-nil, they have the same length,
// and either they are the same map object or their corresponding keys
// (matched using Go equality) map to deeply equal values.
//
// Pointer values are deeply equal if they are equal using Go's == operator
// or if they point to deeply equal values.
//
// Slice values are deeply equal when all of the following are true:
// they are both nil or both non-nil, they have the same length,
// and either they point to the same initial entry of the same underlying array
// (that is, &x[0] == &y[0]) or their corresponding elements (up to length) are deeply equal.
// Note that a non-nil empty slice and a nil slice (for example, []byte{} and []byte(nil))
// are not deeply equal.
//
// Other values - numbers, bools, strings, and channels - are deeply equal
// if they are equal using Go's == operator.
//
// In general DeepEqual is a recursive relaxation of Go's == operator.
// However, this idea is impossible to implement without some inconsistency.
// Specifically, it is possible for a value to be unequal to itself,
// either because it is of func type (uncomparable in general)
// or because it is a floating-point NaN value (not equal to itself in floating-point comparison),
// or because it is an array, struct, or interface containing
// such a value.
// On the other hand, pointer values are always equal to themselves,
// even if they point at or contain such problematic values,
// because they compare equal using Go's == operator, and that
// is a sufficient condition to be deeply equal, regardless of content.
// DeepEqual has been defined so that the same short-cut applies
// to slices and maps: if x and y are the same slice or the same map,
// they are deeply equal regardless of content.
//
// As DeepEqual traverses the data values it may find a cycle. The
// second and subsequent times that DeepEqual compares two pointer
// values that have been compared before, it treats the values as
// equal rather than examining the values to which they point.
// This ensures that DeepEqual terminates.

// DeepEqual has been modified to ignore the monotonic portion of time.Time objects.
func DeepEqual(x, y interface{}) bool {
	if x == nil || y == nil {
		return x == y
	}
	v1 := reflect.ValueOf(x)
	v2 := reflect.ValueOf(y)
	if v1.Type() != v2.Type() {
		return false
	}
	return deepValueEqual(v1, v2, make(map[visit]bool), 0)
}

// Twiddle this flag for the complete spew.SDump of an object
// This is useful when pumped into a diff tool, but likely extreme
// overkill for most uses. The diff that is created by default should
// be enough for the average test.
var superVerbose = false

// AssertDeepEqual fails the test if the two objects do not pass reflect.DeepEqual.
func AssertDeepEqual(t testutils.TestingT, expected, actual interface{}) {
	if !DeepEqual(expected, actual) {
		// The formatting is inspired by stretchr/testify's assert.Equal() output.
		extra := ""
		if doDetailedDiff(expected, actual) {
			e := spewConfig.Sdump(expected)
			a := spewConfig.Sdump(actual)

			diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
				A:        difflib.SplitLines(e),
				B:        difflib.SplitLines(a),
				FromFile: "Expected",
				FromDate: "",
				ToFile:   "Actual",
				ToDate:   "",
				Context:  2,
			})

			extra = "\n\nDiff:\n" + diff
		}

		if superVerbose {
			assert.FailNow(t, fmt.Sprintf("Objects do not match: \na:\n%s\n\nb:\n%s\n%s", spew.Sdump(expected), spew.Sdump(actual), extra))
		} else {
			assert.FailNow(t, fmt.Sprintf("Objects do not match: \na:\n%#v\n\nb:\n%#v\n%s", expected, actual, extra))
		}
	}
}

// doDetailedDiff returns true if doing a detailed diff would help. This means if
// the two objects are the same type and are one of the complicated types:
// e.g. Map, Slice, Struct, etc.
func doDetailedDiff(e, a interface{}) bool {
	if e == nil || a == nil {
		return false
	}

	et := reflect.TypeOf(e)
	ek := et.Kind()
	if ek == reflect.Ptr {
		et = et.Elem()
		ek = et.Kind()
	}
	at := reflect.TypeOf(a)
	ak := at.Kind()
	if ak == reflect.Ptr {
		at = at.Elem()
		ak = at.Kind()
	}

	if et != at {
		return false
	}

	if ek != reflect.Struct && ek != reflect.Map && ek != reflect.Slice && ek != reflect.Array {
		return false
	}
	return true
}

var spewConfig = spew.ConfigState{
	Indent:                  "  ",
	DisablePointerAddresses: true,
	DisableCapacities:       true,
	SortKeys:                true,
}

// AssertCopy is AssertDeepEqual but also checks that none of the direct fields
// have a zero value and none of the direct fields point to the same object.
// This catches regressions where a new field is added without adding that field
// to the Copy method. Arguments must be structs.
func AssertCopy(t testutils.TestingT, a, b interface{}) {
	AssertDeepEqual(t, a, b)

	// Check that all fields are non-zero.
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	assert.Equal(t, va.Type(), vb.Type(), "Arguments are different types.")
	for va.Kind() == reflect.Ptr {
		assert.Equal(t, reflect.Ptr, vb.Kind(), "Arguments are different types (pointer vs. non-pointer)")
		va = va.Elem()
		vb = vb.Elem()
	}
	assert.Equal(t, reflect.Struct, va.Kind(), "Not a struct or pointer to struct.")
	assert.Equal(t, reflect.Struct, vb.Kind(), "Arguments are different types (pointer vs. non-pointer)")
	for i := 0; i < va.NumField(); i++ {
		fa := va.Field(i)
		z := reflect.Zero(fa.Type())
		if reflect.DeepEqual(fa.Interface(), z.Interface()) {
			assert.FailNow(t, fmt.Sprintf("Missing field %q (or set to zero value).", va.Type().Field(i).Name))
		}
		if fa.Kind() == reflect.Map || fa.Kind() == reflect.Ptr || fa.Kind() == reflect.Slice {
			fb := vb.Field(i)
			assert.NotEqual(t, fa.Pointer(), fb.Pointer(), "Field %q not deep-copied.", va.Type().Field(i).Name)
		}
	}
}

// AssertJSONRoundTrip encodes and decodes an object to/from JSON and asserts
// that the result is deep equal to the original. obj must be a pointer.
func AssertJSONRoundTrip(t testutils.TestingT, obj interface{}) {
	val := reflect.ValueOf(obj)
	assert.Equal(t, reflect.Ptr, val.Kind(), "AssertJSONRoundTrip must be passed a pointer.")
	cpyval := reflect.New(val.Elem().Type())
	cpy := cpyval.Interface()
	buf := bytes.Buffer{}
	assert.NoError(t, json.NewEncoder(&buf).Encode(obj))
	assert.NoError(t, json.NewDecoder(&buf).Decode(cpy))
	AssertDeepEqual(t, obj, cpy)
}
