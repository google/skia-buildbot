#!/bin/bash

# --- Configuration ---
DB_HOST="localhost"
DB_PORT="5432"
DB_USER="root"
DB_NAME="demo"

# --- Define the JSON Payload ---
# It's best practice to store large strings like this in a variable
# to keep the SQL statement clean.
ALERT_JSON='{
  "id_as_string": "-1",
  "display_name": "a",
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
  "category": "a",
  "action": "noaction"
}'

# --- Define the other required values ---
SUB_NAME="test_subscription"
SUB_REVISION="initial_rev"

# --- Execute the SQL Insertion using a Here Document (<<EOF) ---
# The -w flag tells psql to prompt for a password if required.
# The -v ON_ERROR_STOP=1 ensures the script stops if the INSERT fails.
psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -v ON_ERROR_STOP=1 -w <<-EOF
    -- Insert a new row into the Alerts table
    INSERT INTO Alerts (
        alert,
        last_modified,
        sub_name,
        sub_revision
    )
    VALUES (
        -- Use the defined shell variable here for the complex JSON
        '${ALERT_JSON}',

        -- Use PostgreSQL's function to get the current Unix timestamp as an integer
        EXTRACT(EPOCH FROM NOW())::INT,

        -- Use the defined shell variables for names/revisions
        '${SUB_NAME}',
        '${SUB_REVISION}'
    );

    -- Optional: Print the inserted row (assuming 'id' is auto-generated)
    SELECT * FROM Alerts ORDER BY id DESC LIMIT 1;
EOF

echo "--- Alert inserted successfully. ---"
