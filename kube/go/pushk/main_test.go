package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestFilter(t *testing.T) {
	unittest.SmallTest(t)
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

func TestImageFromCmdLineImage(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		value         string
		provider      tagProvider
		expected      string
		expectedError bool
		message       string
	}{
		{
			value:         "gcr.io/skia-public/fiddle:2018-...",
			provider:      nil,
			expected:      "gcr.io/skia-public/fiddle:2018-...",
			expectedError: false,
			message:       "already full name",
		},
		{
			value: "fiddle",
			provider: func(name string) ([]string, error) {
				return []string{
					"2018-04-20T21_14_00Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
					"foo",
					"2018-04-20T21_21_48Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
				}, nil
			},
			expectedError: false,
			expected:      "gcr.io/skia-public/fiddle:2018-04-20T21_21_48Z-jcgregorio-f40851bf4611a844bb63b289e91cddc6eba886ae-dirty",
			message:       "tag provider",
		},
		{
			value: "fiddle",
			provider: func(name string) ([]string, error) {
				return []string{}, fmt.Errorf("Failed to find any tags.")
			},
			expected:      "",
			expectedError: true,
			message:       "tag provider",
		},
	}

	for _, tc := range testCases {
		got, err := imageFromCmdLineImage(tc.value, tc.provider)
		if (err != nil) != tc.expectedError {
			t.Errorf("Error condition unexpected: %s, Error expected: %v, %s", err, tc.expectedError, tc.message)

		}
		if want := tc.expected; got != want {
			t.Errorf("Failed case Got %v Want %v: %s", got, want, tc.message)
		}
	}
}
