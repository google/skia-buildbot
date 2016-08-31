// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import (
	"fmt"

	template "github.com/luci/luci-go/common/data/text/templateproto"
)

// JSONObjectMaxLength is the maximum number of bytes that may be present in the
// Object field of a normalized JSONObject.
const JSONObjectMaxLength = 256 * 1024

// JSONNonNormalizedSizeFactor is the excess multiple of JSONObjectMaxLength
// that a non-normalized json object must be smaller than. Otherwise we won't
// attempt to normalize it at all.
const JSONNonNormalizedSizeFactor = 0.1

var jsonNonNormalizedSize int

func init() {
	// can't do this conversion statically because reasons
	siz := JSONObjectMaxLength * (1 + JSONNonNormalizedSizeFactor)
	jsonNonNormalizedSize = int(siz)
}

// Normalize normalizes the JSONObject (ensures it's an object, removes
// whitespace, sorts keys, normalizes Size value, etc.)
func (j *JsonResult) Normalize() error {
	if j == nil {
		return nil
	}
	if j.Object == "" {
		j.Size = 0
		return nil
	}
	if len(j.Object) > jsonNonNormalizedSize {
		return fmt.Errorf(
			"JSONObject.Object length exceeds max non-normalized length: %d > %d (max)",
			len(j.Object), jsonNonNormalizedSize)
	}
	normed, err := template.NormalizeJSON(j.Object, true)
	if err != nil {
		return err
	}
	if len(normed) > JSONObjectMaxLength {
		return fmt.Errorf(
			"JSONObject.Object length exceeds max: %d > %d (max)",
			len(normed), JSONObjectMaxLength)
	}
	j.Object = normed
	j.Size = uint32(len(j.Object))
	return nil
}
