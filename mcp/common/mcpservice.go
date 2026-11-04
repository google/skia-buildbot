// Copyright 2025 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

// McpService provides an interface for defining a mcp service.
type McpService interface {
	// Initialize the service with the provided arguments.
	Init(serviceArgs string) error

	// GetTools returns all the tools supported by the McpService.
	GetTools() []Tool

	// Shutdown implements shutdown procedure for the service.
	Shutdown() error

	// GetResources returns all the resources provided by the McpService.
	GetResources() []Resource
}
