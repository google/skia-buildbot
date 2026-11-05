// Copyright 2024 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package schema

type GraphsShortcutSchema struct {
	ID string `sql:"id TEXT UNIQUE NOT NULL PRIMARY KEY"`

	Graphs string `sql:"graphs TEXT"`
}
