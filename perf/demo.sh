#!/bin/bash

# Script to launch a local copy of CockroachDB for use by Perf in demo mode.

cockroach start-single-node --insecure --listen-addr=localhost:25000 --store=/tmp/cdb &

echo "CREATE DATABASE demo;" | cockroach sql --insecure --host=localhost:25000 

cockroach sql --url=postgresql://root@localhost:25000/demo?sslmode=disable  < ./migrations/cockroachdb/0001_create_initial_tables.up.sql