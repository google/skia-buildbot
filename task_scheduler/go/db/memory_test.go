package db

import "testing"

func TestInMemoryDB(t *testing.T) {
	TestTaskDB(t, NewInMemoryDB())
}

func TestInMemoryTooManyUsers(t *testing.T) {
	TestTooManyUsers(t, NewInMemoryDB())
}

func TestInMemoryConcurrentUpdate(t *testing.T) {
	TestConcurrentUpdate(t, NewInMemoryDB())
}

func TestInMemoryUpdateTasksWithRetries(t *testing.T) {
	TestUpdateTasksWithRetries(t, NewInMemoryDB())
}
