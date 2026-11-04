// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateReviewPrompt(t *testing.T) {
	patch := "diff --git a/file.txt b/file.txt\n+added"
	prompt := createReviewPrompt(patch)
	assert.Contains(t, prompt, patch)
}
