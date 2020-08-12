package firestore

import (
	"testing"

	"go.skia.org/infra/task_scheduler/go/db/shared_tests"
)

func TestModifiedTasksCh(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestModifiedTasksCh(t, d)
}

func TestModifiedJobsCh(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestModifiedJobsCh(t, d)
}

func TestModifiedTaskCommentsCh(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestModifiedTaskCommentsCh(t, d)
}

func TestModifiedTaskSpecCommentsCh(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestModifiedTaskSpecCommentsCh(t, d)
}

func TestModifiedCommitCommentsCh(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	shared_tests.TestModifiedCommitCommentsCh(t, d)
}
