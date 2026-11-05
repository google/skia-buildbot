// Copyright 2016 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package memory

import (
	"testing"

	"go.skia.org/infra/task_scheduler/go/db"
)

// TestCommentBox checks that CommentBox correctly implements CommentDB.
func TestCommentBox(t *testing.T) {
	db.TestCommentDB(t, NewCommentBox())
}
