package pubsub

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/db"
)

func setupTasks(t *testing.T) db.ModifiedTasks {
	unittest.LargeTest(t)
	topic := uuid.New()
	m, err := NewModifiedTasks("fake-project", fmt.Sprintf("modified-tasks-test-%s", topic), "fake-label", nil)
	assert.NoError(t, err)
	return m
}

func TestPubsubModifiedTasks(t *testing.T) {
	m := setupTasks(t)
	db.TestModifiedTasks(t, m)
}

func TestPubsubMultipleTaskModifications(t *testing.T) {
	m := setupTasks(t)
	db.TestMultipleTaskModifications(t, m)
}

func setupJobs(t *testing.T) db.ModifiedJobs {
	unittest.LargeTest(t)
	topic := uuid.New()
	m, err := NewModifiedJobs("fake-project", fmt.Sprintf("modified-jobs-test-%s", topic), "fake-label", nil)
	assert.NoError(t, err)
	return m
}

func TestPubsubModifiedJobs(t *testing.T) {
	m := setupJobs(t)
	db.TestModifiedJobs(t, m)
}

func TestPubsubMultipleJobModifications(t *testing.T) {
	m := setupJobs(t)
	db.TestMultipleJobModifications(t, m)
}

func setupComments(t *testing.T) db.ModifiedComments {
	unittest.LargeTest(t)
	topic1 := fmt.Sprintf("modified-comments-test-tasks-%s", uuid.New())
	topic2 := fmt.Sprintf("modified-comments-test-taskspecs-%s", uuid.New())
	topic3 := fmt.Sprintf("modified-comments-test-commits-%s", uuid.New())
	m, err := NewModifiedComments("fake-project", topic1, topic2, topic3, "fake-label", nil)
	assert.NoError(t, err)
	return m
}

func TestPubsubModifiedComments(t *testing.T) {
	m := setupComments(t)
	db.TestModifiedComments(t, m)
}

func TestPubsubMultipleCommentModifications(t *testing.T) {
	m := setupComments(t)
	db.TestMultipleCommentModifications(t, m)
}
