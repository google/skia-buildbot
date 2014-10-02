package db

import (
	"fmt"
	"os"
	"testing"
)

import (
	// Using 'require' which is like using 'assert' but causes tests to fail.
	assert "github.com/stretchr/testify/require"
)

// Connection string to the local MySQL database for testing.
const (
	MYSQL_DB_OPEN = "%s:%s@tcp(localhost:3306)/skia?parseTime=true"
)

func TestSQLiteVersioning(t *testing.T) {
	// Initialize without argument to test against SQLite3
	Init("")
	assert.False(t, isMySQL)
	testDBVersioning(t)
}

func TestMySQLVersioning(t *testing.T) {
	rwUserPw, rootPw := os.Getenv("MYSQL_TESTING_RWPW"), os.Getenv("MYSQL_TESTING_ROOTPW")
	// Skip this test unless there are environment variables with the rwuser and
	// root password for the local MySQL instance.
	if (rwUserPw == "") || (rootPw == "") {
		t.Skip("Skipping versioning tests against MySQL. Set 'MYSQL_TESTING_ROOTPW' and 'MYSQL_TESTING_RWPW' to enable tests.")
	}

	// Open DB as readwrite user and make sure it fails because of a missing
	// version table.
	// Note: This requires the database to be empty.
	assert.Panics(t, func() {
		Init(fmt.Sprintf(MYSQL_DB_OPEN, "readwrite", rwUserPw))
	})

	// OpenDB as root user and make sure migration works.
	// Initialize to test against local MySQL db
	Init(fmt.Sprintf(MYSQL_DB_OPEN, "root", rootPw))
	assert.True(t, isMySQL)
	testDBVersioning(t)

	// Make sure it doesn't panic for readwrite user after the migration
	assert.NotPanics(t, func() {
		Init(fmt.Sprintf(MYSQL_DB_OPEN, "readwrite", rwUserPw))
	})
}

// Test wether the migration scripts execute.
func testDBVersioning(t *testing.T) {
	// get the DB version
	dbVersion, err := DBVersion()
	assert.Nil(t, err)
	maxVersion := MaxDBVersion()

	// downgrade to 0
	err = Migrate(0)
	assert.Nil(t, err)
	dbVersion, err = DBVersion()
	assert.Nil(t, err)
	assert.Equal(t, 0, dbVersion)

	// upgrade the the latest version
	err = Migrate(maxVersion)
	assert.Nil(t, err)
	dbVersion, err = DBVersion()
	assert.Nil(t, err)
	assert.Equal(t, maxVersion, dbVersion)
}
