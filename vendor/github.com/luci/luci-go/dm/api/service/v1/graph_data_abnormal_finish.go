// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

// CouldRetry returns true iff this status code is retryable.
func (s AbnormalFinish_Status) CouldRetry() bool {
	switch s {
	case AbnormalFinish_FAILED, AbnormalFinish_CRASHED,
		AbnormalFinish_EXPIRED, AbnormalFinish_TIMED_OUT:

		return true
	}
	return false
}
