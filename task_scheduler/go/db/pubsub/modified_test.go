package pubsub

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/db"
)

func setupTasks(t *testing.T) db.ModifiedTasks {
	testutils.LargeTest(t)
	c, err := pubsub.NewClient(context.Background(), "fake-project")
	assert.NoError(t, err)
	topic := uuid.New()
	m, err := NewModifiedTasks(c, fmt.Sprintf("modified-tasks-test-%s", topic), "fake-label")
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
	c, err := pubsub.NewClient(context.Background(), "fake-project")
	assert.NoError(t, err)
	topic := uuid.New()
	m, err := NewModifiedJobs(c, fmt.Sprintf("modified-jobs-test-%s", topic), "fake-label")
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
