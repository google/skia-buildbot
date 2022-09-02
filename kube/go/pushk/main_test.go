package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/util"
)

func TestFilter(t *testing.T) {
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

func Test_byClusterFromChanged(t *testing.T) {
	type args struct {
		gitDir  string
		changed util.StringSet
	}
	tests := []struct {
		name    string
		args    args
		want    map[string][]string
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				gitDir: "/tmp/k8s-config",
				changed: util.StringSet{
					"/tmp/k8s-config/skia-corp/alert-to-pubsub.yaml":    true,
					"/tmp/k8s-config/skia-corp/android-compile-2.yaml":  true,
					"/tmp/k8s-config/skia-public/skottie-internal.yaml": true,
					"/tmp/k8s-config/skia-public/skottie.yaml":          true,
				},
			},
			want: map[string][]string{
				"skia-corp":   {"/tmp/k8s-config/skia-corp/alert-to-pubsub.yaml", "/tmp/k8s-config/skia-corp/android-compile-2.yaml"},
				"skia-public": {"/tmp/k8s-config/skia-public/skottie-internal.yaml", "/tmp/k8s-config/skia-public/skottie.yaml"},
			},
			wantErr: false,
		},
		{
			name: "empty",
			args: args{
				gitDir:  "/tmp/k8s-config",
				changed: util.StringSet{},
			},
			want:    map[string][]string{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := byClusterFromChanged(tt.args.gitDir, tt.args.changed)
			if (err != nil) != tt.wantErr {
				t.Errorf("byClusterFromChanged() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for cluster, files := range tt.want {
					assert.ElementsMatch(t, files, got[cluster])
				}
			}
		})
	}
}
