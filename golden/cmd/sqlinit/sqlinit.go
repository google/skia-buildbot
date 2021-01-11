// The sqlinit executable initializes a database on the production SQL cluster with the appropriate
// tables and schema. It will not modify any table schemas (e.g. add missing
// indexes or change columns). This executable will schedule new automatic backups, so if there
// are existing ones, one may have to drop the old schedules.
// https://www.cockroachlabs.com/docs/v20.2/show-schedules
// https://www.cockroachlabs.com/docs/v20.2/drop-schedules
package main

import (
	"flag"
	"fmt"
	"os/exec"
	"reflect"
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

	sklog.Infof("Initializing tables")
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

	sklog.Infof("Initializing backup schedules")
	out, err = exec.Command("kubectl", "run",
		"gold-cockroachdb-init-"+normalizedDB,
		"--restart=Never", "--image=cockroachdb/cockroach:v20.2.3",
		"--rm", "-it", // -it forces this command to wait until it completes.
		"--", "sql",
		"--insecure", "--host=gold-cockroachdb:26234",
		"--execute="+getSchedules(schema.Tables{}, normalizedDB),
	).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Error while creating tables: %s %s", err, out)
	}
	sklog.Info("Done")
}

type backupCadence struct {
	cadence string
	tables  []string
}

func getSchedules(inputType interface{}, dbName string) string {
	var schedules []*backupCadence

	t := reflect.TypeOf(inputType)
	for i := 0; i < t.NumField(); i++ {
		table := t.Field(i) // Fields of the outer type are expected to be tables.
		if table.Type.Kind() != reflect.Slice {
			panic(`Expected table should be a slice: ` + table.Name)
		}
		cadence, ok := table.Tag.Lookup("sql_backup")
		if !ok {
			panic(`Expected table should have backup cadence. Did you mean "none"? ` + table.Name)
		}
		if cadence == "none" {
			continue
		}
		found := false
		for _, s := range schedules {
			if s.cadence == cadence {
				found = true
				s.tables = append(s.tables, table.Name)
				break
			}
		}
		if found {
			continue
		}
		schedules = append(schedules, &backupCadence{
			cadence: cadence,
			tables:  []string{table.Name},
		})
	}
	body := strings.Builder{}
	for _, s := range schedules {
		schedule := fmt.Sprintf("CREATE SCHEDULE %s_%s\n", dbName, s.cadence)
		tables := "FOR BACKUP TABLE"
		for i, t := range s.tables {
			if i == 0 {
				tables += " "
			} else {
				tables += ", "
			}
			tables += dbName + "." + t
		}
		schedule += tables + "\n"
		schedule += fmt.Sprintf(`INTO 'gs://skia-gold-sql-backups/%s/%s'
    RECURRING '@%s'
    FULL BACKUP ALWAYS;
`, dbName, s.cadence, s.cadence)

		body.WriteString(schedule)
	}
	return body.String()
}
