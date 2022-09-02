package memory

import (
	"testing"

	"go.skia.org/infra/task_scheduler/go/db"
)

// TestCommentBox checks that CommentBox correctly implements CommentDB.
func TestCommentBox(t *testing.T) {
	db.TestCommentDB(t, NewCommentBox())
}
