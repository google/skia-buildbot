package main

import (
	"testing"
)

func TestRun(t *testing.T) {
	cfg := Config{
		InputFile:  "test_input.json",
		OutputFile: "test_output.json",
	}
	err := Run(cfg)
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
	// TODO(eduardoyap): Add more tests for JSON conversion logic
}
