package db

import "testing"

func TestInMemoryTaskDB(t *testing.T) {
	TestTaskDB(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBTooManyUsers(t *testing.T) {
	TestTaskDBTooManyUsers(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBConcurrentUpdate(t *testing.T) {
	TestTaskDBConcurrentUpdate(t, NewInMemoryTaskDB())
}

func TestInMemoryTaskDBUpdateTasksWithRetries(t *testing.T) {
	TestUpdateTasksWithRetries(t, NewInMemoryTaskDB())
}

func TestInMemoryJobDB(t *testing.T) {
	TestJobDB(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBTooManyUsers(t *testing.T) {
	TestJobDBTooManyUsers(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBConcurrentUpdate(t *testing.T) {
	TestJobDBConcurrentUpdate(t, NewInMemoryJobDB())
}

func TestInMemoryJobDBUpdateJobsWithRetries(t *testing.T) {
	TestUpdateJobsWithRetries(t, NewInMemoryJobDB())
}
