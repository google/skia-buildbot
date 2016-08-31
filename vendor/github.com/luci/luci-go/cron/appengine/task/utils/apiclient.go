// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package utils

import (
	"github.com/luci/luci-go/common/errors"
	"google.golang.org/api/googleapi"
)

// IsTransientAPIError returns true if error from Google API client indicates
// an error that can go away on its own in the future.
func IsTransientAPIError(err error) bool {
	if err == nil {
		return false
	}
	apiErr, _ := err.(*googleapi.Error)
	if apiErr == nil {
		return true // failed to get HTTP code => connectivity error => transient
	}
	return apiErr.Code >= 500 || apiErr.Code == 429 || apiErr.Code == 0
}

// WrapAPIError wraps error from Google API client in transient wrapper if
// necessary.
func WrapAPIError(err error) error {
	if IsTransientAPIError(err) {
		return errors.WrapTransient(err)
	}
	return err
}
