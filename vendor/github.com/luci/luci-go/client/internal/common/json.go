// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package common

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReadJSONFile reads a file and decode it as JSON.
func ReadJSONFile(filePath string, object interface{}) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", filePath, err)
	}
	defer f.Close()
	if err = json.NewDecoder(f).Decode(object); err != nil {
		return fmt.Errorf("failed to decode %s: %s", filePath, err)
	}
	return nil
}

// WriteJSONFile writes object as json encoded into filePath with 2 spaces
// indentation. File permission is set to user only.
func WriteJSONFile(filePath string, object interface{}) error {
	d, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode %s: %s", filePath, err)
	}

	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %s: %s", filePath, err)
	}
	defer f.Close()
	if _, err := f.Write(d); err != nil {
		return fmt.Errorf("failed to write %s: %s", filePath, err)
	}
	return nil
}
