package main

import (
	"database/sql"
	"flag"
	"io/ioutil"
	"log"
	"os/exec"
	"time"

	"go.skia.org/infra/go/sklog"

	// Make sure the postgress driver is loaded.
	_ "github.com/lib/pq"
)

func main() {
	flag.Parse()
	err := exec.Command("killall", "-9", "cockroach").Run()
	if err != nil {
		sklog.Warning("Attempted to stop previous cockroach instances failed. Probably were none.")
	}

	out, err := exec.Command("cockroach", "version").CombinedOutput()
	if err != nil {
		sklog.Fatalf("Do you have 'cockroach' on your path? %s: %s", err, out)
	}
	sklog.Infof("cockroach version: \n%s", out)

	tmpDir, err := ioutil.TempDir("", "cockroach-db")
	if err != nil {
		sklog.Fatalf("Could not make tempdir: %s", err)
	}

	err = exec.Command("cockroach",
		"start-single-node", "--insecure", "--listen-addr=localhost:26257",
		"--http-addr=localhost:8080", "--background",
		"--store="+tmpDir,
	).Start()

	if err != nil {
		sklog.Fatalf("Could not start local cockroach version %s: %s", err)
	}

	// Wait for DB to come up.
	time.Sleep(3 * time.Second)

	sklog.Infof("Check out localhost:8080 and %s for storage", tmpDir)

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		`--execute=CREATE DATABASE IF NOT EXISTS demo_gold_db;`).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Could not create database: %s %s", err, out)
	}

	out, err = exec.Command("cockroach", "sql", "--insecure", "--host=localhost:26257",
		"--database=demo_gold_db", // Connect to demo_gold_db that we just made
		`--execute=
CREATE TABLE IF NOT EXISTS TraceValues ( -- 60 bytes per data point
	trace_hash BYTES, -- SHA256 hash of the trace string
	options_hash BYTES, -- SHA256 hash of the options string
	commit_number INT4,
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	source_file_id BYTES NOT NULL, -- SHA256 hash of the source file id.
	PRIMARY KEY (trace_id, commit_number)
);`,
		`--execute=CREATE TABLE IF NOT EXISTS Commits (
  commit_number INT4 PRIMARY KEY, -- The commit_number; a monotonically increasing number as we follow master branch through time.
  git_hash STRING NOT NULL,
  commit_time TIMESTAMP WITH TIME ZONE NOT NULL,
  author STRING,
  subject STRING
);`,
		`--execute=CREATE TABLE IF NOT EXISTS TraceIDs  (
	trace_hash BYTES PRIMARY KEY, -- SHA256 hash of the trace string
	trace_id STRING -- The trace id itself, e.g. ,color mode=RGB,device=walleye,
);`,
		`--execute=CREATE TABLE IF NOT EXISTS OptionIDs  (
	options_hash BYTES PRIMARY KEY, -- SHA256 hash of the options string
	options_id STRING -- The options "id" itself, e.g. ,ext=png,
);`,
	).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Could not create tables: %s %s", err, out)
	}

	// TODO(kjlubick) https://www.cockroachlabs.com/docs/stable/comment-on.html#add-a-comment-to-a-column

	db, err := sql.Open("postgres",
		"postgresql://localhost:26257/demo_gold_db?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}

	// TODO Try inserting trace_ids using UPSERT
	defer db.Close()
	sklog.Infof("Done")
}
