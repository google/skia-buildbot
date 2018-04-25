package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestFilter(t *testing.T) {
	testutils.SmallTest(t)
	testCases := []struct {
		value    []string
		expected []string
		hasError bool
		message  string
	}{
		{
			value:    []string{},
			expected: nil,
			hasError: true,
			message:  "Empty",
		},
		{
			value: []string{
				"invalid tag",
			},
			expected: nil,
			hasError: true,
			message:  "No valid tags",
		},
		{
			value:    []string{"2018-04-20T21_21_48Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty "},
			expected: []string{"2018-04-20T21_21_48Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty "},
			hasError: false,
			message:  "Single",
		},
		{
			value: []string{
				"2018-04-20T21_21_48Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
				"2018-04-20T21_14_00Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
			},
			expected: []string{
				"2018-04-20T21_14_00Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
				"2018-04-20T21_21_48Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
			},
			hasError: false,
			message:  "Multiple, Sort",
		},
		{
			value: []string{
				"invalid tag",
				"2018-04-20T21_21_48Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
				"another invalid tag",
				"2018-04-20T21_14_00Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
				"final invalid tag",
			},
			expected: []string{
				"2018-04-20T21_14_00Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
				"2018-04-20T21_21_48Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
			},
			hasError: false,
			message:  "Multiple, Sort, Strip invalid",
		},
	}

	for _, tc := range testCases {
		got, err := filter(tc.value)
		want := tc.expected
		gotError := err != nil
		wantError := tc.hasError
		assert.Equal(t, wantError, gotError, "hasError: "+tc.message)
		assert.Equal(t, want, got, tc.message)
	}
}
