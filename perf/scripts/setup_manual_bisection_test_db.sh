#!/bin/bash
# setup_manual_bisection_test_db.sh
# Initializes the local demo Spanner database, runs ingestion, and inserts mock data
# for manual testing of the `maybe_trigger_bisection` workflow.

set -e

DB_HOST="localhost"
DB_PORT="5432"
DB_USER="root"
DB_NAME="demo"

# 1. Verify Spanner adapter is running on port 5432
echo "Verifying PostgreSQL adapter on port ${DB_PORT}..."
if ! nc -z "${DB_HOST}" "${DB_PORT}"; then
  echo "Error: Could not connect to PostgreSQL adapter on ${DB_HOST}:${DB_PORT}."
  echo "Please start the Spanner emulator / adapter"
  echo "(e.g., by running run_with_spanner.sh or similar)."
  exit 1
fi

echo "PostgreSQL adapter is running."

# 2. Re-initialize database schema (Fresh database)
echo "Initializing demo Spanner schema..."
bazelisk run --config=remote -c dbg //perf/go/initdemo:initdemo -- \
  --database_url="postgresql://${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

# 3. Ingest demo commits and traces
echo "Ingesting demo traces..."
../_bazel_bin/perf/go/perfserver/perfserver_/perfserver ingest \
  --local \
  --config_filename=./configs/demo_spanner.json \
  --connection_string="postgresql://${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

# 4. Insert customized Alert, Regression, and AnomalyGroup objects
echo "Inserting manual testing records (Alert, Regressions2, AnomalyGroup)..."

ALERT_JSON='{
  "id_as_string": "-1",
  "display_name": "Manual Test Alert",
  "query": "benchmark=speedometer",
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
  "category": "manual",
  "action": "bisect"
}'

SUB_NAME="test_subscription"
SUB_REVISION="123456"

# Fixed UUIDs so they are easy to query and pass to Temporal
REGRESSION_ID="967b5610-d88e-47fc-8f68-7b91be3a7d4a"
ANOMALY_GROUP_ID="a9d70df7-ff99-4720-9988-cb9470987114"

psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -v ON_ERROR_STOP=1 <<-EOF
    -- Safe idempotent cleanup of previous test records (demo DB only)
    DELETE FROM AnomalyGroups WHERE id = '${ANOMALY_GROUP_ID}';
    DELETE FROM Regressions2 WHERE id = '${REGRESSION_ID}';
    DELETE FROM Subscriptions WHERE name = '${SUB_NAME}' AND revision = '${SUB_REVISION}';
    DELETE FROM Alerts WHERE sub_name = '${SUB_NAME}' AND sub_revision = '${SUB_REVISION}';
    DELETE FROM Autobisections
    WHERE anomaly_group_id = '${ANOMALY_GROUP_ID}'
       OR job_id = 'fedcba98-7654-3210-fedc-ba9876543210';

    -- Create Subscription and Alert
    INSERT INTO Alerts (
        alert,
        last_modified,
        sub_name,
        sub_revision
    )
    VALUES (
        '${ALERT_JSON}',
        EXTRACT(EPOCH FROM NOW())::INT,
        '${SUB_NAME}',
        '${SUB_REVISION}'
    );

    INSERT INTO Subscriptions (
        name,
        revision,
        bug_component,
        bug_priority,
        bug_severity,
        bug_cc_emails,
        contact_email,
        is_active
    )
    VALUES (
        '${SUB_NAME}',
        '${SUB_REVISION}',
        '98765',
        2,
        2,
        ARRAY['manual-test@example.com'],
        'contact@example.com',
        true
    );

    -- Insert Regressions2 object with valid frame/paramset
    INSERT INTO Regressions2 (
        id,
        commit_number,
        prev_commit_number,
        display_commit_number,
        alert_id,
        sub_name,
        bug_id,
        creation_time,
        median_before,
        median_after,
        is_improvement,
        cluster_type,
        cluster_summary,
        frame,
        trace_id,
        triage_status,
        triage_message
    ) VALUES (
        '${REGRESSION_ID}',
        100,
        90,
        100,
        (SELECT id FROM Alerts ORDER BY id DESC LIMIT 1),
        '${SUB_NAME}',
        0,
        NOW(),
        15.5,
        22.3,
        false,
        'high',
        '{}'::jsonb,
        '{"dataframe": {
            "paramset": {
                "bot": ["linux-perf"],
                "benchmark": ["speedometer"],
                "test": ["runsperminute"],
                "stat": ["value"],
                "subtest_1": ["speedometer"],
                "story": ["speedometer"],
                "improvement_direction": ["UP"]
            }
        }}'::jsonb,
        '\x00'::bytea,
        'untriaged',
        ''
    );

    -- Insert AnomalyGroup
    INSERT INTO AnomalyGroups (
        id,
        action,
        creation_time,
        common_rev_start,
        common_rev_end,
        anomaly_ids,
        culprit_ids,
        group_meta_data
    ) VALUES (
        '${ANOMALY_GROUP_ID}',
        'BISECT',
        NOW(),
        90,
        100,
        ARRAY['${REGRESSION_ID}'],
        NULL,
        '{"subscription_name": "${SUB_NAME}",
          "subscription_revision": "${SUB_REVISION}",
          "benchmark_name": "speedometer"}'::jsonb
    );
EOF

echo "--------------------------------------------------------"
echo "Mock database populated successfully!"
echo "Created Anomaly Group ID: ${ANOMALY_GROUP_ID}"
echo "--------------------------------------------------------"
