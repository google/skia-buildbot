package db

import "testing"

func TestInMemoryDB(t *testing.T) {
	TestDB(t, NewInMemoryDB())
}

func TestInMemoryTooManyUsers(t *testing.T) {
	TestTooManyUsers(t, NewInMemoryDB())
}
