// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package endpoints

// ProjectBoundMessage describes an object that is bound to a Project
// namespace.
//
// This is intended to be implemented by project-bound protobufs.
type ProjectBoundMessage interface {
	// GetMessageProject returns the Project to which this message is bound.
	GetMessageProject() string
}
