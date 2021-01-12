// The sqlinit executable creates a database on the production SQL cluster with the appropriate
// schema. It is idempotent and will not modify any tables (e.g. add missing indexes or change
// columns).
package main

import (
	"flag"
	"os/exec"
	"strings"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql/schema"
)

func main() {
	dbName := flag.String("db_name", "", "name of database to init")

	flag.Parse()
	if *dbName == "" {
		sklog.Fatalf("Must supply db_name")
	}
	// Both k8s and cockroachdb expect database names to be lowercase.
	normalizedDB := strings.ToLower(*dbName)

	sklog.Infof("Creating database %s", normalizedDB)
	out, err := exec.Command("kubectl", "run",
		"gold-cockroachdb-init-"+normalizedDB,
		"--restart=Never", "--image=cockroachdb/cockroach:v20.2.3",
		"--rm", "-it", // -it forces this command to wait until it completes.
		"--", "sql",
		"--insecure", "--host=gold-cockroachdb:26234",
		"--execute=CREATE DATABASE IF NOT EXISTS "+normalizedDB,
	).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Error while creating database %s: %s %s", normalizedDB, err, out)
	}

	sklog.Infof("Creating tables")
	out, err = exec.Command("kubectl", "run",
		"gold-cockroachdb-init-"+normalizedDB,
		"--restart=Never", "--image=cockroachdb/cockroach:v20.2.3",
		"--rm", "-it", // -it forces this command to wait until it completes.
		"--", "sql",
		"--insecure", "--host=gold-cockroachdb:26234", "--database="+normalizedDB,
		"--execute="+schema.Schema,
	).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Error while creating tables: %s %s", err, out)
	}
	sklog.Info("Done")
}
