// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package bootstrapResult defines a common way to express the result of
// bootstrapping a command via JSON.
package bootstrapResult

import (
	"encoding/json"
	"os"
)

// Result is the bootstrap result data.
type Result struct {
	// ReturnCode is the process' return code.
	ReturnCode int `json:"return_code"`
	// Command, is present, is the bootstrapped command that was run.
	Command []string `json:"command,omitempty"`
}

// WriteJSON writes Result as a JSON document to the specified path.
func (r *Result) WriteJSON(path string) error {
	fd, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	return json.NewEncoder(fd).Encode(r)
}
