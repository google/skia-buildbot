package assertdeep

import (
	"fmt"
	"reflect"

	"github.com/davecgh/go-spew/spew"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sktest"
)

// Twiddle this flag for the complete spew.SDump of an object
// This is useful when pumped into a diff tool, but likely extreme
// overkill for most uses. The diff that is created by default should
// be enough for the average test.
var superVerbose = false

// Equal fails the test if the two objects do not pass a modified version of reflect.DeepEqual.
// The modification (see infra/go/deepequal) will use the .Equal() method if defined on the type.
// This is necessary for comparing structs that might differ on unexported fields, but be
// practically the same. This concretely comes up often when deserialzing a time.Time, but applies
// in a few other situations as well.
func Equal(t sktest.TestingT, expected, actual interface{}) {
	diff := Diff(expected, actual)
	if diff != "" {
		require.FailNow(t, diff)
	}
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

// Diff determines whether the two values are DeepEqual.  If they are not
// equal, it returns a string diff of the two, otherwise it returns the empty
// string.
func Diff(x, y interface{}) string {
	if deepequal.DeepEqual(x, y) {
		return ""
	}
	// The formatting is inspired by stretchr/testify's require.Equal() output.
	extra := ""
	if doDetailedDiff(x, y) {
		e := spewConfig.Sdump(x)
		a := spewConfig.Sdump(y)

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
		return fmt.Sprintf("Objects do not match: \na:\n%s\n\nb:\n%s\n%s", spew.Sdump(x), spew.Sdump(y), extra)
	} else {
		return fmt.Sprintf("Objects do not match: \na:\n%#v\n\nb:\n%#v\n%s", x, y, extra)
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
