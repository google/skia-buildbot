// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import "fmt"

// MinimumActivationTokenLength is the minimum number of bytes in an appropriate
// ExecutionToken.
const MinimumActivationTokenLength = 32

// Normalize returns an error iff the ActivateExecutionReq has bad form (nils,
// insufficient activation token length, etc.
func (a *ActivateExecutionReq) Normalize() error {
	if err := a.Auth.Normalize(); err != nil {
		return err
	}
	if len(a.ExecutionToken) < MinimumActivationTokenLength {
		return fmt.Errorf("insufficiently long ExecutionToken: %d",
			len(a.ExecutionToken))
	}
	return nil
}
