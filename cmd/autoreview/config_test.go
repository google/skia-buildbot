package main

import (
	"os"
	"testing"
	"time"

	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestParse(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name     string
		args     []string
		expected *Config
	}{
		{
			name: "default values",
			args: []string{"autoreview"},
			expected: &Config{
				GCPProject:   "skia-infra-corp",
				Location:     "global",
				Model:        "gemini-3.5-flash",
				BaseCommit:   "HEAD~1",
				ContextLines: 10,
				Timeout:      time.Minute,
				Verbose:      false,
				ShowWarnings: true,
				ShowLGTM:     true,
			},
		},
		{
			name: "custom values",
			args: []string{
				"autoreview",
				"--verbose",
				"--base-commit", "main",
				"--model", "test-model",
				"--project", "test-project",
				"--timeout", "5m",
				"--show-warnings=false",
				"--show-lgtm=false",
			},
			expected: &Config{
				GCPProject:   "test-project",
				Location:     "global",
				Model:        "test-model",
				BaseCommit:   "main",
				ContextLines: 10,
				Timeout:      5 * time.Minute,
				Verbose:      true,
				ShowWarnings: false,
				ShowLGTM:     false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			cfg, err := Parse()
			if err != nil {
				t.Fatalf("Parse() returned unexpected error: %v", err)
			}
			assertdeep.Equal(t, tt.expected, cfg)
		})
	}
}
