package main

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os/exec"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
	"go.skia.org/infra/golden/go/types"

	// Make sure the postgreSQL driver is loaded.
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
CREATE TABLE IF NOT EXISTS TraceValues (
	trace_hash BYTES, -- MD5 hash of the trace string
	commit_number INT4,
	digest BYTES NOT NULL, -- MD5 hash of the pixel data
	options_hash BYTES, -- MD5 hash of the options string
	source_file_hash BYTES NOT NULL, -- MD5 hash of the source file name
	PRIMARY KEY (trace_hash, commit_number)
);`,
		`--execute=CREATE TABLE IF NOT EXISTS Commits (
  commit_number INT4 PRIMARY KEY, -- The commit_number; a monotonically increasing number as we follow master branch through time.
  git_hash STRING NOT NULL,
  commit_time TIMESTAMP WITH TIME ZONE NOT NULL,
  author STRING,
  subject STRING
);`,
		`--execute=CREATE TABLE IF NOT EXISTS TraceIDs  (
	trace_hash BYTES PRIMARY KEY, -- MD5 hash of the keys (which defines the trace to be unique)
	keys JSONB NOT NULL -- The trace's keys, e.g. {"color mode":"RGB", "device":"walleye"}
);`,
		`--execute=CREATE TABLE IF NOT EXISTS OptionIDs  (
	options_hash BYTES PRIMARY KEY, -- MD5 hash of the options 
	options JSONB NOT NULL -- The trace keys, e.g. {"ext":"png"}
);`,
		`--execute=CREATE TABLE IF NOT EXISTS SourceFiles (
	source_file_hash BYTES PRIMARY KEY, -- The MD5 hash of the source file name
	source_file STRING NOT NULL  -- The full name of the source file, e.g. gs://bucket/2020/01/02/03/15/foo.json
);`,
	).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Could not create tables: %s %s", err, out)
	}

	// TODO(kjlubick) https://www.cockroachlabs.com/docs/stable/comment-on.html#add-a-comment-to-a-column

	db, err := sql.Open("postgres",
		"postgresql://root@localhost:26257/demo_gold_db?sslmode=disable")
	if err != nil {
		log.Fatal("error connecting to the database: ", err)
	}

	ctx := context.Background()
	const fakeFile = "skia-gold-flutter/dm-json-v1/2020/03/31/23/d14a301e419af7f3eff7cc3a49bf936c75d2b2f0/waterfall/1585696758/dm-1585696758433097948.json"

	sourceFileHash := md5.Sum([]byte(fakeFile))

	for _, tp := range data_kitchen_sink.MakeTraces() {
		trace := tp.Trace
		keysHash, keysJSON, err := serializeMap(trace.Keys())
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}
		optsHash, optsJSON, err := serializeMap(trace.Options())
		if err != nil {
			sklog.Fatalf("Should never happen: %s", err)
		}

		for commitNum := 0; commitNum < len(trace.Digests); commitNum++ {
			digestBytes, err := digestToBytes(trace.Digests[commitNum])
			if err != nil {
				sklog.Fatalf("Invalid digest: %s", err)
			}

			_, err = db.ExecContext(ctx,
				`INSERT INTO TraceIDs (trace_hash, keys) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				keysHash, keysJSON)
			if err != nil {
				sklog.Fatalf("Could not insert keys %s - %s: %s", keysHash, trace.Keys(), err)
			}

			_, err = db.ExecContext(ctx,
				`INSERT INTO OptionIDs (options_hash, options) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				optsHash, optsJSON)
			if err != nil {
				sklog.Fatalf("Could not insert options %s - %s: %s", optsHash, trace.Options(), err)
			}

			_, err = db.ExecContext(ctx,
				`INSERT INTO SourceFiles (source_file_hash, source_file) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				sourceFileHash[:], fakeFile)
			if err != nil {
				sklog.Fatalf("Could not insert source file %s - %s: %s", sourceFileHash, fakeFile, err)
			}

			_, err = db.ExecContext(ctx,
				`UPSERT INTO TraceValues (trace_hash, commit_number, digest, options_hash, source_file_hash)
VALUES ($1, $2, $3, $4, $5)`,
				keysHash, commitNum, digestBytes, optsHash, sourceFileHash[:])
			if err != nil {
				sklog.Fatalf("Could not insert data for trace %s commit %d: %s", tp.ID, commitNum, err)
			}

		}
		sklog.Infof("Wrote trace %s (the long way)", tp.ID)

	}

	defer db.Close()
	sklog.Infof("Done")
}

func serializeMap(m map[string]string) ([]byte, string, error) {
	str, err := json.Marshal(m)
	if err != nil {
		return nil, "", err
	}
	h := md5.Sum(str)
	return h[:], string(str), err
}

func digestToBytes(d types.Digest) ([]byte, error) {
	return hex.DecodeString(string(d))
}
