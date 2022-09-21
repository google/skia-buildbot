package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBazelTargetToDockerTag(t *testing.T) {
	tc := map[string]string{
		"//task_scheduler:task_scheduler_jc_container": "bazel/task_scheduler:task_scheduler_jc_container",
	}
	for input, expect := range tc {
		require.Equal(t, expect, bazelTargetToDockerTag(input))
	}
}
