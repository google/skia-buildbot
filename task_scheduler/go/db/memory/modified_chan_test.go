package memory

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db/shared_tests"
)

func TestModifiedTasksCh(t *testing.T) {
	unittest.SmallTest(t)
	d := NewInMemoryDB()
	shared_tests.TestModifiedTasksCh(t, d)
}

func TestModifiedJobsCh(t *testing.T) {
	unittest.SmallTest(t)
	d := NewInMemoryDB()
	shared_tests.TestModifiedJobsCh(t, d)
}

func TestModifiedTaskCommentsCh(t *testing.T) {
	unittest.SmallTest(t)
	d := NewInMemoryDB()
	shared_tests.TestModifiedTaskCommentsCh(t, d)
}

func TestModifiedTaskSpecCommentsCh(t *testing.T) {
	unittest.SmallTest(t)
	d := NewInMemoryDB()
	shared_tests.TestModifiedTaskSpecCommentsCh(t, d)
}

func TestModifiedCommitCommentsCh(t *testing.T) {
	unittest.SmallTest(t)
	d := NewInMemoryDB()
	shared_tests.TestModifiedCommitCommentsCh(t, d)
}
