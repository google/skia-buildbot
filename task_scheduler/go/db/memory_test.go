package db

import "testing"

func TestInMemoryTaskDB(t *testing.T) {
	TestTaskDB(t, NewInMemoryTaskDB())
}

func TestInMemoryTooManyUsers(t *testing.T) {
	TestTooManyUsers(t, NewInMemoryTaskDB())
}

func TestInMemoryConcurrentUpdate(t *testing.T) {
	TestConcurrentUpdate(t, NewInMemoryTaskDB())
}

func TestInMemoryUpdateTasksWithRetries(t *testing.T) {
	TestUpdateTasksWithRetries(t, NewInMemoryTaskDB())
}
