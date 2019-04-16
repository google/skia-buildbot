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
	var values []string
	m := &multiString{
		values: &values,
	}
	addAndCheck := func(newVal string, expect []string) {
		assert.NoError(t, m.Set(newVal))
		deepequal.AssertDeepEqual(t, expect, *m.values)
		deepequal.AssertDeepEqual(t, expect, values)
		// Sanity check.
		deepequal.AssertDeepEqual(t, []string{"mydefault", "mydefault2"}, defaults)
	}

	addAndCheck("alpha", []string{"alpha"})
	addAndCheck("beta,gamma", []string{"alpha", "beta", "gamma"})
	addAndCheck("delta", []string{"alpha", "beta", "gamma", "delta"})

	// Test MultiStringFlagVar behavior.
	values = nil
	m = newMultiString(&values, defaults)
	deepequal.AssertDeepEqual(t, defaults, *m.values)
	deepequal.AssertDeepEqual(t, defaults, values)

	addAndCheck("alpha", []string{"alpha"})
	addAndCheck("beta,gamma", []string{"alpha", "beta", "gamma"})
	addAndCheck("delta", []string{"alpha", "beta", "gamma", "delta"})

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
	newMultiString(&values, nil)
	assert.Nil(t, values)
}
