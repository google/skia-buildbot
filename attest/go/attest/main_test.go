package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidImageRegex(t *testing.T) {
	valid := func(imageID string) {
		require.True(t, validImageRegex.MatchString(imageID))
	}

	valid("gcr.io/skia-public/task-scheduler-be@sha256:d5062719ba4240c2d5b5beb31882b8beba584a6e218464365e7b117404ca8992")
}
