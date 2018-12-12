package pubsub

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

func setupTasks(t *testing.T) db.ModifiedTasks {
	testutils.LargeTest(t)
	topic := uuid.New()
	m, err := NewModifiedTasks(fmt.Sprintf("modified-tasks-test-%s", topic), "fake-label", nil)
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
	testutils.LargeTest(t)
	topic := uuid.New()
	m, err := NewModifiedJobs(fmt.Sprintf("modified-jobs-test-%s", topic), "fake-label", nil)
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
