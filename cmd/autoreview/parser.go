// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"regexp"
	"strings"
)

var lgtmRegex = regexp.MustCompile(
	`(?i){\s*"lgtm"\s*:\s*"?\s*(true|false)\s*"?\s*}`,
)

// parseLgtm searches for the latest JSON in the format {"lgtm": true/false}
// inside the reviewText and returns true if lgtm is true, false otherwise.
// If it cannot find a matching JSON, it returns an error.
func parseLgtm(reviewText string) (bool, error) {
	matches := lgtmRegex.FindAllStringSubmatch(reviewText, -1)
	if len(matches) == 0 {
		return false, fmt.Errorf("could not parse LGTM status from review text")
	}

	// Get the last matched group
	latestMatch := matches[len(matches)-1]

	// Group 1 is (true|false)
	lgtmValue := strings.ToLower(latestMatch[1])
	return lgtmValue == "true", nil
}
