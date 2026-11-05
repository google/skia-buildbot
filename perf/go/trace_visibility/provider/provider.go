// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package provider

import "context"

// Provider defines the interface for a visibility source.
type Provider interface {
	// GetExpectedRules fetches and returns the list of expected rules.
	GetExpectedRules(ctx context.Context) (map[string]bool, error)
}
