#!/bin/bash

 perf-tool  --config_filename=./configs/ct-prod.json --connection_string=postgresql://root@localhost:26257/ct?sslmode=disable database backup alerts --out /tmp/alerts.zip

 perf-tool --config_filename=./configs/ct-prod.json --connection_string=postgresql://root@localhost:26257/ct?sslmode=disable --backup_to_date=2020-01-01 database backup regressions --out /tmp/regressions.zip


  perf-tool --config_filename=./configs/cdb-ct-prod.json --connection_string=postgresql://root@localhost:26257/ct?sslmode=disable database restore alerts --in /tmp/alerts.zip

 perf-tool  --config_filename=./configs/cdb-ct-prod.json --connection_string=postgresql://root@localhost:26257/ct?sslmode=disable database restore regressions --in /tmp/regressions.zip