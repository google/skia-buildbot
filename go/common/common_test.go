package common

import (
	"flag"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMultiString(t *testing.T) {
	unittest.SmallTest(t)

	// Test basic operation.
	defaults := []string{"mydefault", "mydefault2"}
	var values []string
	m := &multiString{
		values: &values,
	}
	addAndCheck := func(newVal string, expect []string, expectStr string) {
		require.NoError(t, m.Set(newVal))
		deepequal.AssertDeepEqual(t, expect, *m.values)
		deepequal.AssertDeepEqual(t, expect, values)
		// Sanity check.
		deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, defaults)
		require.Equal(t, expectStr, m.String())
	}

	addAndCheck("alpha", []string{"alpha"}, "alpha")
	addAndCheck("beta,gamma", []string{"alpha", "beta", "gamma"}, "alpha,beta,gamma")
	addAndCheck("delta", []string{"alpha", "beta", "gamma", "delta"}, "alpha,beta,gamma,delta")

	// Test MultiStringFlagVar behavior.
	values = nil
	m = newMultiString(&values, defaults)
	deepequal.AssertDeepEqual(t, defaults, *m.values)
	deepequal.AssertDeepEqual(t, defaults, values)

	addAndCheck("alpha", []string{"alpha"}, "alpha")
	addAndCheck("beta,gamma", []string{"alpha", "beta", "gamma"}, "alpha,beta,gamma")
	addAndCheck("delta", []string{"alpha", "beta", "gamma", "delta"}, "alpha,beta,gamma,delta")

	// Sanity check.
	deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, defaults)

	// Verify that changing the defaults does not change the flag values.
	values = nil
	m = newMultiString(&values, defaults)
	defaults[0] = "replaced"
	deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, *m.values)
	deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, values)

	// Verify that it's okay to pass nil for the defaults.
	values = nil
	m = newMultiString(&values, nil)
	require.Nil(t, values)
	require.Equal(t, "", m.String())

	// This is the code from the flag package which caused a crash without a
	// nil check in String().
	NewMultiStringFlag("myflag", nil, "Use --myflag")
	myflag := flag.Lookup("myflag")
	require.NotNil(t, myflag)

	typ := reflect.TypeOf(myflag.Value)
	var z reflect.Value
	if typ.Kind() == reflect.Ptr {
		z = reflect.New(typ.Elem())
	} else {
		z = reflect.Zero(typ)
	}
	require.Equal(t, "", z.Interface().(flag.Value).String())
}
