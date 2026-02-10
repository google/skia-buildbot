package semver

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	test := func(t *testing.T, re *regexp.Regexp, version, expectVersion string, expect interface{}) {
		t.Run(version, func(t *testing.T) {
			ints, actualVersion, err := parseVersion(re, version)
			if expectErr, ok := expect.(error); ok {
				require.EqualError(t, err, expectErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, expectVersion, actualVersion)
				require.Equal(t, expect, ints)
			}
		})
	}
	t.Run("major only", func(t *testing.T) {
		re := regexp.MustCompile(`^(\d+).0.0$`)
		test(t, re, "1.0.0", "1.0.0", []int{1})
		test(t, re, "1.2.0", "1.2.0", ErrNoMatch)
		test(t, re, "1.2.3", "1.2.3", ErrNoMatch)
	})
	t.Run("major and minor", func(t *testing.T) {
		re := regexp.MustCompile(`^(\d+)\.(\d+).0$`)
		test(t, re, "1.0.0", "1.0.0", []int{1, 0})
		test(t, re, "1.2.0", "1.2.0", []int{1, 2})
		test(t, re, "1.2.3", "1.2.3", ErrNoMatch)
	})
	t.Run("all fields", func(t *testing.T) {
		re := regexp.MustCompile(`^(\d+)\.(\d+).(\d+)$`)
		test(t, re, "1.0.0", "1.0.0", []int{1, 0, 0})
		test(t, re, "1.2.0", "1.2.0", []int{1, 2, 0})
		test(t, re, "1.2.3", "1.2.3", []int{1, 2, 3})
	})
	t.Run("partial match", func(t *testing.T) {
		re := regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)`)
		test(t, re, "upstream/v1.2.3", "v1.2.3", []int{1, 2, 3})
		test(t, re, "upstream/v1.2.3.3845", "v1.2.3", []int{1, 2, 3})
	})
	t.Run("nested capture groups", func(t *testing.T) {
		re := regexp.MustCompile(`^upstream/(v(\d+)\.(\d+)\.(\d+))$`)
		test(t, re, "upstream/v1.2.3", "v1.2.3", []int{1, 2, 3})
	})
	t.Run("no match", func(t *testing.T) {
		re := regexp.MustCompile(`^(\d+)\.(\d+).0$`)
		test := func(version string) {
			t.Run("version", func(t *testing.T) {
				ints, actualVersion, err := parseVersion(re, version)
				require.EqualError(t, err, ErrNoMatch.Error())
				require.Empty(t, actualVersion)
				require.Nil(t, ints)
			})
		}
		test("blah blah")
		test("12345")
		test("v1.0.0")
		test("1.2.1")
	})
}

func TestVersion_Compare(t *testing.T) {
	test := func(a, b []int, expect int) {
		op := "=="
		if expect == -1 {
			op = ">"
		} else if expect == 1 {
			op = "<"
		}
		t.Run(fmt.Sprintf("%v%s%v", a, op, b), func(t *testing.T) {
			vA := &Version{version: a}
			vB := &Version{version: b}
			require.Equal(t, expect, vA.Compare(vB))
		})
	}
	test([]int{}, []int{}, 0)
	test([]int{}, []int{1}, 1)
	test([]int{1}, []int{}, -1)
	test([]int{1}, []int{1}, 0)
	test([]int{0}, []int{1}, 1)
	test([]int{1}, []int{0}, -1)
	test([]int{1, 1}, []int{1, 0}, -1)
	test([]int{1}, []int{1, 0}, 1)
	test([]int{1, 0}, []int{1}, -1)
}
