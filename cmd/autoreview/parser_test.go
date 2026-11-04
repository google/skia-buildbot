// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLgtm(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  bool
		expectErr bool
	}{
		{
			name:      "lowercase true",
			input:     `{"lgtm": true}`,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "lowercase false",
			input:     `{"lgtm": false}`,
			expected:  false,
			expectErr: false,
		},
		{
			name:      "uppercase true",
			input:     `{"LGTM": TRUE}`,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "with spaces",
			input:     `{  "lgtm"  :  true  }`,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "with string true",
			input:     `{"lgtm": "true"}`,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "with string FALSE",
			input:     `{"LGTM": "FALSE"}`,
			expected:  false,
			expectErr: false,
		},
		{
			name:      "latest match wins",
			input:     `{"lgtm": false} ... some text ... {"lgtm": true}`,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "latest match wins 2",
			input:     `{"lgtm": true} ... some text ... {"lgtm": false}`,
			expected:  false,
			expectErr: false,
		},
		{
			name:      "no match",
			input:     `{"something": true}`,
			expected:  false,
			expectErr: true,
		},
		{
			name:      "case insensitive lgtm key",
			input:     `{"lGtM": true}`,
			expected:  true,
			expectErr: false,
		},
		{
			name:      "complex text around",
			input:     "This looks good.\n```json\n{\"lgtm\": true}\n```",
			expected:  true,
			expectErr: false,
		},
		{
			name:      "newlines inside json spaces",
			input:     "{\n  \"LGTM\" : \n  true\n}",
			expected:  true,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lgtm, err := parseLgtm(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, lgtm)
			}
		})
	}
}
