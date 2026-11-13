// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// devseed adds the demo alert config and its sheriff (subscription) to the local demo database,
// so that "perfserver frontend --do_clustering=true" detects regressions that perfinsights can
// serve. It does what perf/scripts/add_demo_alert_to_demo_db.sh does, without needing psql, and it
// is idempotent.
//
// It only writes to a database named "demo" on a loopback host.
package main

import (
	"context"
	"flag"
	"time"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/insights/localonly"
)

const (
	subName     = "test_subscription"
	subRevision = "123456"

	// demoAlert is the alert config of add_demo_alert_to_demo_db.sh: a StepFit alert on the demo
	// benchmark that detects the step in the demo data. Unlike the script, it also sets sub_name
	// and sub_revision in the JSON: Perf reads an alert's subscription from the JSON, not from the
	// Alerts columns, and stores detected regressions with an empty sub_name without it.
	demoAlert = `{
  "id_as_string": "-1",
  "display_name": "Demo 8888 alert",
  "query": "benchmark=8888",
  "alert": "",
  "issue_tracker_component": "",
  "interesting": 10,
  "bug_uri_template": "",
  "algo": "stepfit",
  "step": "absolute",
  "state": "ACTIVE",
  "owner": "user@example.com",
  "step_up_only": false,
  "direction": "BOTH",
  "radius": 1,
  "k": 1,
  "group_by": "",
  "sparse": false,
  "minimum_num": 0,
  "category": "demo",
  "action": "noaction",
  "sub_name": "test_subscription",
  "sub_revision": "123456"
}`
)

var connectionString = flag.String("connection_string", "postgresql://root@127.0.0.1:5432/demo?sslmode=disable", "Connection string of the local demo database.")

func main() {
	flag.Parse()
	if err := run(); err != nil {
		sklog.Fatal(err)
	}
}

func run() error {
	if err := localonly.CheckDemoDatabase(*connectionString); err != nil {
		return skerr.Wrapf(err, "devseed only writes to the local demo database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, *connectionString)
	if err != nil {
		return skerr.Wrapf(err, "connecting to the demo database")
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			sklog.Warningf("Failed to close the connection: %s", err)
		}
	}()

	var alerts int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM Alerts WHERE sub_name = $1`, subName).Scan(&alerts); err != nil {
		return skerr.Wrapf(err, "counting the demo alerts")
	}
	if alerts == 0 {
		if _, err := conn.Exec(ctx,
			`INSERT INTO Alerts (alert, last_modified, sub_name, sub_revision) VALUES ($1, $2, $3, $4)`,
			demoAlert, time.Now().Unix(), subName, subRevision); err != nil {
			return skerr.Wrapf(err, "inserting the demo alert")
		}
		sklog.Infof("Inserted the demo alert for %q.", subName)
	} else {
		// Rewrites the config, which repairs demo alerts created without sub_name in the JSON.
		if _, err := conn.Exec(ctx,
			`UPDATE Alerts SET alert = $1, last_modified = $2 WHERE sub_name = $3`,
			demoAlert, time.Now().Unix(), subName); err != nil {
			return skerr.Wrapf(err, "updating the demo alert")
		}
		sklog.Infof("Updated the demo alert for %q.", subName)
	}

	var subs int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM Subscriptions WHERE name = $1 AND revision = $2`, subName, subRevision).Scan(&subs); err != nil {
		return skerr.Wrapf(err, "counting the demo subscriptions")
	}
	if subs == 0 {
		if _, err := conn.Exec(ctx,
			`INSERT INTO Subscriptions (name, revision, bug_component, bug_priority, bug_severity, bug_cc_emails, contact_email, is_active)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			subName, subRevision, "98765", 2, 2, []string{"cc@example.com", "cc1@example.com"}, "contact@mail.com", true); err != nil {
			return skerr.Wrapf(err, "inserting the demo subscription")
		}
		sklog.Infof("Inserted the demo subscription %q.", subName)
	} else {
		sklog.Infof("The demo subscription %q already exists.", subName)
	}
	return nil
}
