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
const MYSQL_DB_OPEN = "root:%s@tcp(localhost:3306)/skia?parseTime=true"

func TestSQLiteVersioning(t *testing.T) {
	// Initialize without argument to test against SQLite3
	Init("")
	assert.False(t, isMySQL)
	testDBVersioning(t)
}

func TestMySQLVersioning(t *testing.T) {
	// Skip this test unless there is an environment variable with the root
	// password for the local MySQL instance.
	password := os.Getenv("MYSQL_TESTING_ROOTPW")
	if password == "" {
		t.Skip("Skipping versioning tests against MySQL. Set 'MYSQL_TESTING_ROOTPW' to enable tests.")
	}

	// Initialize to test against local MySQL db
	Init(fmt.Sprintf(MYSQL_DB_OPEN, password))
	assert.True(t, isMySQL)
	testDBVersioning(t)
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
