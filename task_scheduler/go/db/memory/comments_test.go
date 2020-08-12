package memory

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db/shared_tests"
)

// TestCommentBox checks that CommentBox correctly implements CommentDB.
func TestCommentBox(t *testing.T) {
	unittest.SmallTest(t)
	shared_tests.TestCommentDB(t, NewCommentBox())
}
