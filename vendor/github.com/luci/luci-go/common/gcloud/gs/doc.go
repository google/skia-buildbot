// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package gs implements a versatile Google Storage client on top of the
// standard Google Storage Go API. It adds:
//	- The ability to read from specific byte offsets.
//	- Exponential backoff retries on transient errors.
//	- Logging
//	- The ability to easily stub a Google Storage interface.
package gs
