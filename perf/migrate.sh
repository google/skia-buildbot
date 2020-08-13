#!/bin/bash

perf-tool database backup alerts      --local --config_filename=./configs/flutter-engine.json \
  --connection_string=postgresql://root@localhost:26257/flutter_engine?sslmode=disable --out=/tmp/alerts.dat
perf-tool database backup regressions --local --config_filename=./configs/flutter-engine.json \
  --connection_string=postgresql://root@localhost:26257/flutter_engine?sslmode=disable --out=/tmp/regressions.dat \
  --backup_to_date=2020-07-01

perf-tool database restore alerts      --local --config_filename=./configs/cdb-flutter-engine.json \
  --connection_string=postgresql://root@localhost:26257/flutter_engine?sslmode=disable --in=/tmp/alerts.dat
perf-tool database restore regressions --local --config_filename=./configs/cdb-flutter-engine.json \
  --connection_string=postgresql://root@localhost:26257/flutter_engine?sslmode=disable --in=/tmp/regressions.dat