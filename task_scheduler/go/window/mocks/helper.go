// Copyright 2021 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mocks

import (
	time "time"

	mock "github.com/stretchr/testify/mock"
)

// AllInclusiveWindow returns a Window which includes everything.
func AllInclusiveWindow() *Window {
	w := &Window{}
	w.On("EarliestStart").Return(time.Time{})
	w.On("Start", mock.Anything).Return(time.Time{})
	w.On("StartTimesByRepo").Return(map[string]time.Time{})
	w.On("TestCommit", mock.Anything, mock.Anything).Return(true)
	w.On("TestCommitHash", mock.Anything, mock.Anything).Return(true, nil)
	w.On("TestTime", mock.Anything, mock.Anything).Return(true)
	w.On("Update").Return(nil)
	w.On("UpdateWithTime", mock.Anything).Return(nil)
	return w
}
