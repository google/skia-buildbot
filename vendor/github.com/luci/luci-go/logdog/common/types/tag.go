// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package types

import (
	"fmt"
)

const (
	// MaxTagKeySize is the maximum number of bytes allowed in a tag's key
	// parameter.
	MaxTagKeySize = 64

	// MaxTagValueSize is the maximum number of bytes allowed in a tag's value
	// parameter.
	MaxTagValueSize = 4096
)

// ValidateTag returns an error if a tag contains a nonconfirming value.
func ValidateTag(k, v string) error {
	if err := StreamName(k).Validate(); err != nil {
		return fmt.Errorf("invalid tag key: %s", err)
	}
	if len(k) > MaxTagKeySize {
		return fmt.Errorf("tag key exceeds maximum size (%d > %d)", len(k), MaxTagKeySize)
	}
	if len(v) > MaxTagValueSize {
		return fmt.Errorf("tag value exceeds maximum size (%d > %d)", len(v), MaxTagValueSize)
	}
	return nil
}
