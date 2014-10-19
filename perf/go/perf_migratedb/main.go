package main

// Executes database migrations to the latest target version. In production this
// requires the root password for MySQL. The user will be prompted for that so
// it is not entered via the command line.

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/perf/go/db"
)

func main() {
	defaultConnStr := strings.Replace(db.DB_CONN_TMPL, "%s", "root", 1)

	// flags
	dbConnString := flag.String("db_conn_string", defaultConnStr, "\n\tDatabase string to open connect to the MySQL database. "+
		"\n\tNeeds to follow the format of the golang-mysql driver (https://github.com/go-sql-driver/mysql."+
		"\n\tIf the string contains %s the user will be prompted to enter a password which will then be used for subtitution.")

	flag.Parse()
	defer glog.Flush()

	var connectionStr = *dbConnString

	// if it contains formatting information read the password from stdin.
	if strings.Contains(connectionStr, "%s") {
		glog.Infof("Using connection string: %s", connectionStr)
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter password for MySQL: ")
		password, err := reader.ReadString('\n')
		if err != nil {
			glog.Fatalf("Unable to read password. Error: %s", err.Error())
		}
		connectionStr = fmt.Sprintf(connectionStr, strings.TrimRight(password, "\n"))
	}

	conf := &database.DatabaseConfig{
		MySQLString:    connectionStr,
		MigrationSteps: db.MigrationSteps(),
	}
	vdb := database.NewVersionedDB(conf)

	// Get the current database version
	maxDBVersion := vdb.MaxDBVersion()
	glog.Infof("Latest database version: %d", maxDBVersion)

	dbVersion, err := vdb.DBVersion()
	if err != nil {
		glog.Fatalf("Unable to retrieve database version. Error: %s", err.Error())
	}
	glog.Infof("Current database version: %d", dbVersion)

	if dbVersion < maxDBVersion {
		glog.Infof("Migrating to version: %d", maxDBVersion)
		err = vdb.Migrate(maxDBVersion)
		if err != nil {
			glog.Fatalf("Unable to retrieve database version. Error: %s", err.Error())
		}
	}

	glog.Infoln("Database migration finished.")
}
