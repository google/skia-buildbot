package database

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/util"
)

const (
	// Template for DB connection strings.
	DB_CONN_TMPL = "%s:%s@tcp(%s:%d)/%s?parseTime=true"

	// Name of the root user.
	USER_ROOT = "root"

	// Name of the readwrite user.
	USER_RW = "readwrite"
)

var (
	// Flags
	dbHost *string
	dbPort *int
	dbUser *string
	dbName *string
)

// SetupFlags adds command-line flags for the database.
func SetupFlags(defaultHost string, defaultPort int, defaultUser, defaultDatabase string) {
	dbHost = flag.String("db_host", defaultHost, "Hostname of the MySQL database server.")
	dbPort = flag.Int("db_port", defaultPort, "Port number of the MySQL database.")
	dbUser = flag.String("db_user", defaultUser, "MySQL user name.")
	dbName = flag.String("db_name", defaultDatabase, "Name of the MySQL database.")
}

// checkFlags returns an error if the command-line flags have not been set.
func checkFlags() error {
	if dbHost == nil || dbPort == nil || dbUser == nil || dbName == nil {
		return fmt.Errorf(
			"One or more of the required command-line flags was not set. " +
				"Did you call forget to call database.SetupFlags?")
	}
	return nil
}

// PromptForPassword prompts for a password.
func PromptForPassword() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter password for MySQL user %s: ", *dbUser)
	pw, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("Failed to get password: %v", err)
	}
	pw = strings.Trim(pw, "\n")
	return pw, nil
}

// ConfigFromFlags obtains a DatabaseConfig based on parsed command-line flags.
// If local is true, the DB host is overridden.
func ConfigFromFlags(password string, local bool, m []MigrationStep) (*DatabaseConfig, error) {
	if err := checkFlags(); err != nil {
		return nil, err
	}
	// Override the DB host in local mode.
	useHost := *dbHost
	if local {
		useHost = "localhost"
	}

	usePassword := password
	// Prompt for password if necessary.
	if usePassword == "" && !local {
		var err error
		usePassword, err = PromptForPassword()
		if err != nil {
			return nil, err
		}
	}
	return NewDatabaseConfig(*dbUser, usePassword, useHost, *dbPort, *dbName, m), nil
}

// ConfigFromFlagsAndMetadata obtains a DatabaseConfig based on a combination
// of parsed command-line flags and metadata when not running in local mode.
func ConfigFromFlagsAndMetadata(local bool, m []MigrationStep) (*DatabaseConfig, error) {
	if err := checkFlags(); err != nil {
		return nil, err
	}
	// If not in local mode, get the password from metadata.
	password := ""
	if !local {
		key := ""
		if *dbUser == USER_RW {
			key = metadata.DATABASE_RW_PASSWORD
		} else if *dbUser == USER_ROOT {
			key = metadata.DATABASE_ROOT_PASSWORD
		}
		if key == "" {
			return nil, fmt.Errorf("Unknown user %s; could not obtain password from metadata.", *dbUser)
		}
		var err error
		password, err = metadata.ProjectGet(key)
		if err != nil {
			return nil, fmt.Errorf("Failed to find metadata. Use 'local' flag when running locally.")
		}
	}
	return ConfigFromFlags(password, local, m)
}

// Config information to create a database connection.
type DatabaseConfig struct {
	MySQLString    string
	MigrationSteps []MigrationStep
}

// NewDatabaseConfig constructs a DatabaseConfig from the given options.
func NewDatabaseConfig(user, password, host string, port int, database string, m []MigrationStep) *DatabaseConfig {
	return &DatabaseConfig{
		MySQLString:    fmt.Sprintf(DB_CONN_TMPL, user, password, host, port, database),
		MigrationSteps: m,
	}
}

// Single step to migrated from one database version to the next and back.
type MigrationStep struct {
	MySQLUp   []string
	MySQLDown []string
}

// Database handle to send queries to the underlying database.
type VersionedDB struct {
	// Database intance that is backed by MySQL.
	DB *sql.DB

	// List of migration steps for this database.
	migrationSteps []MigrationStep
}

// Init must be called once before DB is used.
//
// Since it used glog, make sure it is also called after flag.Parse is called.
func NewVersionedDB(conf *DatabaseConfig) *VersionedDB {
	// If there is a connection string then connect to the MySQL server.
	// This is for testing only. In production we get the relevant information
	// from the metadata server.
	var err error
	var DB *sql.DB = nil

	glog.Infoln("Opening SQL database.")
	if DB, err = sql.Open("mysql", conf.MySQLString); err == nil {
		glog.Infoln("Sending Ping.")
		err = DB.Ping()
	}

	if err != nil {
		glog.Fatalln("Failed to open connection to SQL server:", err)
	}

	result := &VersionedDB{
		DB:             DB,
		migrationSteps: conf.MigrationSteps,
	}

	// Make sure the migration table exists.
	if err := result.checkVersionTable(); err != nil {
		// We are using panic() instead of Fataln() to be able to trap this
		// in tests and make sure it fails when no version table exists.
		glog.Errorln("Unable to create version table.")
		panic("Attempt to create version table returned: " + err.Error())
	}
	glog.Infoln("Version table OK.")

	// Ping the database occasionally to keep the connection fresh.
	go func() {
		c := time.Tick(1 * time.Minute)
		for _ = range c {
			if err := result.DB.Ping(); err != nil {
				glog.Warningln("Database failed to respond:", err)
			}
			glog.Infof("db: Successful ping")
		}
	}()

	return result
}

// Close the underlying database.
func (vdb *VersionedDB) Close() error {
	return vdb.DB.Close()
}

// Migrates the database to the specified target version. Use DBVersion() to
// retrieve the current version of the database.
func (vdb *VersionedDB) Migrate(targetVersion int) (rv error) {
	if (targetVersion < 0) || (targetVersion > vdb.MaxDBVersion()) {
		glog.Fatalf("Target db version must be in range: [0 .. %d]", vdb.MaxDBVersion())
	}

	currentVersion, err := vdb.DBVersion()
	if err != nil {
		return err
	}

	if targetVersion == currentVersion {
		return nil
	}

	// start a transaction
	txn, err := vdb.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { rv = CommitOrRollback(txn, rv) }()

	// run through the transactions
	runSteps := vdb.getMigrations(currentVersion, targetVersion)
	if len(runSteps) == 0 {
		glog.Fatalln("Unable to find migration steps.")
	}

	for _, step := range runSteps {
		for _, stmt := range step {
			glog.Infoln("EXECUTING: \n", stmt)
			if _, err = txn.Exec(stmt); err != nil {
				return err
			}
		}
	}

	// update the dbversion table
	if err = vdb.setDBVersion(txn, targetVersion); err != nil {
		return err
	}

	return nil
}

// Returns the current version of the database. It assumes that the
// Migrate function has already been called and the version table has been
// created in the database.
func (vdb *VersionedDB) DBVersion() (int, error) {
	stmt := `
		SELECT version
		FROM sk_db_version
		WHERE id=1`

	var version int
	err := vdb.DB.QueryRow(stmt).Scan(&version)
	return version, err
}

// Returns the highest version currently available.
func (vdb *VersionedDB) MaxDBVersion() int {
	return len(vdb.migrationSteps)
}

// Returns an error if the version table does not exist.
func (vdb *VersionedDB) checkVersionTable() error {
	// Check if the table exists in MySQL.
	stmt := "SHOW TABLES LIKE 'sk_db_version'"

	var temp string
	err := vdb.DB.QueryRow(stmt).Scan(&temp)
	if err != nil {
		// See if we can create the version table.
		return vdb.ensureVersionTable()
	}

	return nil
}

func (vdb *VersionedDB) setDBVersion(txn *sql.Tx, newDBVersion int) error {
	stmt := `REPLACE INTO sk_db_version (id, version, updated) VALUES(1, ?, ?)`
	_, err := txn.Exec(stmt, newDBVersion, time.Now().Unix())
	return err
}

func (vdb *VersionedDB) ensureVersionTable() (rv error) {
	txn, err := vdb.DB.Begin()
	defer func() { rv = CommitOrRollback(txn, rv) }()

	if err != nil {
		fmt.Errorf("Unable to start database transaction. %s", err)
	}

	stmt := `CREATE TABLE IF NOT EXISTS sk_db_version (
			id         INTEGER      NOT NULL PRIMARY KEY,
			version    INTEGER      NOT NULL,
			updated    BIGINT       NOT NULL
		)`
	if _, err = txn.Exec(stmt); err != nil {
		return fmt.Errorf("Creating version table failed: %s", err)
	}

	stmt = "SELECT COUNT(*) FROM sk_db_version"
	var count int
	if err = txn.QueryRow(stmt).Scan(&count); err != nil {
		return fmt.Errorf("Unable to read version table: %s", err)
	}

	// In both cases we want the transaction to roll back.
	if count == 0 {
		err = vdb.setDBVersion(txn, 0)
	} else if count > 1 {
		err = fmt.Errorf("Version table contains more than one row.")
	}

	return err
}

// Returns the SQL statements base on whether we are using MySQL and the
// current and target DB version.
// This function assumes that currentVersion != targetVersion.
func (vdb *VersionedDB) getMigrations(currentVersion int, targetVersion int) [][]string {
	inc := util.SignInt(targetVersion - currentVersion)
	idx := currentVersion
	if inc < 0 {
		idx = currentVersion - 1
	}
	delta := util.AbsInt(targetVersion - currentVersion)
	result := make([][]string, 0, delta)

	for i := 0; i < delta; i++ {
		var temp []string
		switch {
		// using mysqlp
		case (inc > 0):
			temp = vdb.migrationSteps[idx].MySQLUp
		case (inc < 0):
			temp = vdb.migrationSteps[idx].MySQLDown
		}
		result = append(result, temp)
		idx += inc
	}
	return result
}

// Tx wraps the Commit and Rollback methods of a database transaction.
type Tx interface {
	Commit() error
	Rollback() error
}

// CommitOrRollback is a function which commits or rolls back a database
// transaction, depending on whether or not the function returned an error,
// and logs any errors it encounters. Use it like this:
//
// defer func() { rv = CommitOrRollback(tx, rv) }
//
func CommitOrRollback(tx Tx, err error) error {
	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return fmt.Errorf("%v; failed to rollback: %v", err, err2)
		} else {
			return fmt.Errorf("%v; transaction rolled back.", err)
		}
	} else {
		return tx.Commit()
	}
}
