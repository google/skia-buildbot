package buildbot

/*
	Store/Retrieve buildbot data in a database.
*/

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/jmoiron/sqlx"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/metadata"
)

const (
	// Key of the password for the readwrite user.
	METADATA_KEY = "readwrite"

	// Path where the SQLite database is stored when running locally.
	SQLITE_DB_PATH = "./testing.db"

	DATABASE = "buildbot"

	// Template to generate the database connection string in production.
	// The IP address of the database is found here:
	//    https://console.developers.google.com/project/31977622648/sql/instances/skia-master-db/overview
	// And 3306 is the default port for MySQL.
	DB_CONN_TMPL = "%s:%s@tcp(173.194.253.125:3306)/%s?parseTime=true"

	// Username of the read/write user.
	RW_USER = "readwrite"

	TABLE_BUILDS          = "builds"
	TABLE_BUILD_REVISIONS = "buildRevisions"
	TABLE_BUILD_STEPS     = "buildSteps"
)

var (
	DB *sqlx.DB = nil
)

// Setup the database to be shared across the app.
func InitDB(conf *database.DatabaseConfig) error {
	vdb := database.NewVersionedDB(conf)
	dbVersion, err := vdb.DBVersion()
	if err != nil {
		return fmt.Errorf("Could not determine database version: %v", err)
	}
	maxDBVersion := vdb.MaxDBVersion()
	if dbVersion < maxDBVersion {
		glog.Infof("Migrating DB to version: %d", maxDBVersion)
		if err = vdb.Migrate(maxDBVersion); err != nil {
			return fmt.Errorf("Could not migrate DB: %v", err)
		}
	}
	if err = vdb.Close(); err != nil {
		return fmt.Errorf("Could not close database: %v", err)
	}
	if conf.MySQLString != "" {
		DB, err = sqlx.Open("mysql", conf.MySQLString)
	} else {
		DB, err = sqlx.Open("sqlite3", conf.SQLiteFilePath)
	}
	if err != nil {
		return fmt.Errorf("Failed to open database: %v", err)
	}
	return nil
}

// Returns the DB connection string for running in production where a
// metadata server is available. If 'local' is true it will always return
// "" (empty string). When used with Init() this will cause it to use a
// local SQLite database. If it's not local and the meta data server is
// unreachable it will terminate.
func ProdDatabaseConfig(local bool) *database.DatabaseConfig {
	mysqlStr := ""
	sqlitePath := SQLITE_DB_PATH

	// We are in the production environment, so we look up the parameters.
	if !local {
		//  First, get the password from the metadata server.
		// See https://developers.google.com/compute/docs/metadata#custom.
		password, err := metadata.Get(METADATA_KEY)
		if err != nil {
			glog.Fatalf("Failed to find metadata. Use 'local' flag when running locally.")
		}
		mysqlStr, sqlitePath = fmt.Sprintf(DB_CONN_TMPL, RW_USER, password, DATABASE), ""
	}

	return &database.DatabaseConfig{
		MySQLString:    mysqlStr,
		SQLiteFilePath: sqlitePath,
		MigrationSteps: migrationSteps,
	}
}

// Returns a DB connection string for running a local testing MySQL instance.
func localMySQLTestDatabaseConfig(user, password string) *database.DatabaseConfig {
	mysqlStr := fmt.Sprintf("%s:%s@/sk_testing", user, password)
	return &database.DatabaseConfig{
		MySQLString:    mysqlStr,
		SQLiteFilePath: "",
		MigrationSteps: migrationSteps,
	}
}

var v1_up = []string{
	`CREATE TABLE IF NOT EXISTS builds (
		builder     VARCHAR(100) NOT NULL,
		master      VARCHAR(100) NOT NULL,
		number      INT          NOT NULL,
		gotRevision VARCHAR(40),
		branch      VARCHAR(100) NOT NULL,
		results     INT,
		buildslave  VARCHAR(100) NOT NULL,
		started     DOUBLE,
		finished    DOUBLE,
		properties  TEXT,
		CONSTRAINT pk_builderMasterNumber PRIMARY KEY (builder,master,number)
	)`,
	`CREATE TABLE IF NOT EXISTS buildRevisions (
		revision VARCHAR(40)  NOT NULL,
		builder  VARCHAR(100) NOT NULL,
		master   VARCHAR(100) NOT NULL,
		number   INT          NOT NULL,
		CONSTRAINT pk_revisionBuild PRIMARY KEY (revision,builder,master,number),
		FOREIGN KEY (builder,master,number) REFERENCES builds(builder,master,number) ON DELETE CASCADE ON UPDATE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS buildSteps (
		builder      VARCHAR(100) NOT NULL,
		master       VARCHAR(100) NOT NULL,
		buildNumber  INT          NOT NULL,
		name         VARCHAR(100) NOT NULL,
		results      INT,
		number       INT          NOT NULL,
		started      DOUBLE,
		finished     DOUBLE,
		FOREIGN KEY (builder,master,buildNumber) REFERENCES builds(builder,master,number) ON DELETE CASCADE ON UPDATE CASCADE
	)`,
}

var v1_down = []string{
	`DROP TABLE IF EXISTS buildSteps`,
	`DROP TABLE IF EXISTS buildRevisions`,
	`DROP TABLE IF EXISTS builds`,
}

// Define the migration steps.
// Note: Only add to this list, once a step has landed in version control it
// must not be changed.
var migrationSteps = []database.MigrationStep{
	// version 1
	{
		MySQLUp:    v1_up,
		MySQLDown:  v1_down,
		SQLiteUp:   v1_up,
		SQLiteDown: v1_down,
	},
}
