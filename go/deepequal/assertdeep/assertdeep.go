package assertdeep

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
)

// Equal fails the test if the two objects do not pass reflect.DeepEqual.
func Equal(t sktest.TestingT, expected, actual interface{}) {
	assert.Equal(t, expected, actual)
}

// Copy is Equal but also checks that none of the direct fields
// have a zero value and none of the direct fields point to the same object.
// This catches regressions where a new field is added without adding that field
// to the Copy method. Arguments must be structs. Does not check private fields.
func Copy(t sktest.TestingT, a, b interface{}) {
	Equal(t, a, b)

	// Check that all fields are non-zero.
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	require.Equal(t, va.Type(), vb.Type(), "Arguments are different types.")
	for va.Kind() == reflect.Ptr {
		require.Equal(t, reflect.Ptr, vb.Kind(), "Arguments are different types (pointer vs. non-pointer)")
		va = va.Elem()
		vb = vb.Elem()
	}
	require.Equal(t, reflect.Struct, va.Kind(), "Not a struct or pointer to struct.")
	require.Equal(t, reflect.Struct, vb.Kind(), "Arguments are different types (pointer vs. non-pointer)")
	for i := 0; i < va.NumField(); i++ {
		fa := va.Field(i)
		z := reflect.Zero(fa.Type())
		if !fa.CanInterface() || !z.CanInterface() {
			sklog.Errorf("Cannot Interface() field %q; skipping", va.Type().Field(i).Name)
			continue
		}
		if reflect.DeepEqual(fa.Interface(), z.Interface()) {
			require.FailNow(t, fmt.Sprintf("Missing field %q (or set to zero value).", va.Type().Field(i).Name))
		}
		if fa.Kind() == reflect.Map || fa.Kind() == reflect.Ptr || fa.Kind() == reflect.Slice {
			fb := vb.Field(i)
			require.NotEqual(t, fa.Pointer(), fb.Pointer(), "Field %q not deep-copied.", va.Type().Field(i).Name)
		}
	}
}

// JSONRoundTripEqual encodes and decodes an object to/from JSON and asserts
// that the result is deep equal to the original. obj must be a pointer.
func JSONRoundTripEqual(t sktest.TestingT, obj interface{}) {
	val := reflect.ValueOf(obj)
	require.Equal(t, reflect.Ptr, val.Kind(), "JSONRoundTripEqual must be passed a pointer.")
	cpyval := reflect.New(val.Elem().Type())
	cpy := cpyval.Interface()
	buf := bytes.Buffer{}
	require.NoError(t, json.NewEncoder(&buf).Encode(obj))
	require.NoError(t, json.NewDecoder(&buf).Decode(cpy))
	Equal(t, obj, cpy)
}
