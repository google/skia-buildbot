package common

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestMultiString(t *testing.T) {
	testutils.SmallTest(t)

	// Test basic operation.
	defaults := []string{"mydefault", "mydefault2"}
	m := &multiString{
		values: &defaults,
	}
	addAndCheck := func(newVal string, expect []string) {
		assert.NoError(t, m.Set(newVal))
		deepequal.AssertDeepEqual(t, expect, *m.values)
		// Sanity check.
		deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, defaults)
	}
	deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, *m.values)

	addAndCheck("alpha", []string{"alpha"})
	addAndCheck("beta,gamma", []string{"alpha", "beta", "gamma"})
	addAndCheck("delta", []string{"alpha", "beta", "gamma", "delta"})

	// Test MultiStringFlagVar behavior.
	var myValues []string
	m = newMultiString(&myValues, defaults)
	deepequal.AssertDeepEqual(t, defaults, *m.values)
	deepequal.AssertDeepEqual(t, defaults, myValues)

	addAndCheck("alpha", []string{"alpha"})
	addAndCheck("beta,gamma", []string{"alpha", "beta", "gamma"})
	addAndCheck("delta", []string{"alpha", "beta", "gamma", "delta"})

	// Sanity check.
	deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, defaults)

	// Verify that changing the defaults does not change the flag values.
	myValues = nil
	m = newMultiString(&myValues, defaults)
	defaults[0] = "replaced"
	deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, *m.values)
	deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, myValues)

	// Verify that it's okay to pass nil for the defaults.
	var nilValues []string
	newMultiString(&nilValues, nil)
	assert.Nil(t, nilValues)
}
