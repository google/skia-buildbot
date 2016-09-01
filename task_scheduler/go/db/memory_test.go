package db

import "testing"

func TestInMemoryDB(t *testing.T) {
	TestDB(t, NewInMemoryDB())
}

func TestInMemoryTooManyUsers(t *testing.T) {
	TestTooManyUsers(t, NewInMemoryDB())
}

func TestInMemoryConcurrentUpdate(t *testing.T) {
	TestConcurrentUpdate(t, NewInMemoryDB())
}

func TestInMemoryUpdateWithRetries(t *testing.T) {
	TestUpdateWithRetries(t, NewInMemoryDB())
}
