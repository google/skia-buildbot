package login

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitAuthAllowList(t *testing.T) {

	type testCase struct {
		Input           string
		ExpectedDomains map[string]bool
		ExpectedEmails  map[string]bool
	}

	tests := []testCase{
		{
			Input: "google.com chromium.org skia.org",
			ExpectedDomains: map[string]bool{
				"google.com":   true,
				"chromium.org": true,
				"skia.org":     true,
			},
			ExpectedEmails: map[string]bool{},
		},
		{
			Input: "google.com chromium.org skia.org service-account@proj.iam.gserviceaccount.com",
			ExpectedDomains: map[string]bool{
				"google.com":   true,
				"chromium.org": true,
				"skia.org":     true,
			},
			ExpectedEmails: map[string]bool{
				"service-account@proj.iam.gserviceaccount.com": true,
			},
		},
		{
			Input:           "user@example.com service-account@proj.iam.gserviceaccount.com",
			ExpectedDomains: map[string]bool{},
			ExpectedEmails: map[string]bool{
				"user@example.com": true,
				"service-account@proj.iam.gserviceaccount.com": true,
			},
		},
	}

	for _, tc := range tests {
		d, e := splitAuthAllowList(tc.Input)
		assert.Equal(t, tc.ExpectedDomains, d)
		assert.Equal(t, tc.ExpectedEmails, e)
	}
}
