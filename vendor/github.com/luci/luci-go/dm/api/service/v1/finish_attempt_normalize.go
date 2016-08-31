// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import "errors"

// Normalize returns an error iff the ActivateExecutionReq has bad form (nils,
// insufficient activation token length, etc.
func (a *FinishAttemptReq) Normalize() error {
	if err := a.Auth.Normalize(); err != nil {
		return err
	}
	if err := a.Data.Normalize(); err != nil {
		return err
	}
	if a.Data == nil || len(a.Data.Object) == 0 {
		return errors.New("data field is required")
	}
	return nil
}
