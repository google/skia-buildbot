#!/bin/bash
# An example migration script.

perf-tool database backup alerts      --local --config_filename=./configs/nano.json \
  --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --out=/tmp/alerts.dat
perf-tool database backup regressions --local --config_filename=./configs/nano.json \
  --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --out=/tmp/regressions.dat \
  --backup_to_date=2020-01-01

perf-tool database restore alerts      --local --config_filename=./configs/cdb-nano.json \
  --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --in=/tmp/alerts.dat
perf-tool database restore regressions --local --config_filename=./configs/cdb-nano.json \
  --connection_string=postgresql://root@localhost:26257/skia?sslmode=disable --in=/tmp/regressions.dat