include ../make/bazel.mk

.PHONY: build
build: ./modules/json/index.ts
	$(BAZEL) build --config=mayberemote //perf/...

# Requires the CockroachDB emulator to be running.
generate_sql_schema:
	cd ./go/sql; go run ./tosql
	go run ./go/sql/exportschema --out ./go/sql/expectedschema/schema.json

run_cockroachdb_emulators:
	../scripts/run_emulators/run_emulators start

.PHONY: docs
docs:
	$(BAZEL) run --config=mayberemote //perf/go/perf-tool:perf-tool -- markdown > PERFTOOL.md
	$(BAZEL) run --config=mayberemote //perf/go/perfserver --  markdown > PERFSERVER.md

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //perf:push_perfserver

.PHONY: push
push:
	$(BAZEL) run --config=mayberemote //perf:pushk_perfserver

.PHONY: release_backup
release_backup:
	$(BAZEL) run --config=mayberemote //perf:push_perf-cockroachdb-backup

.PHONY: push_backup
push_backup: release_backup
	$(BAZEL) run --config=mayberemote //perf:pushk_perf-cockroachdb-backup

# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/perf/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles)
	go generate ./go/ts

.PHONY: test
test:
	$(BAZEL) test --config=mayberemote //perf/...

.PHONY: demo_alert-config-sk
demo_alert-config-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/alert-config-sk:demo_page_server

.PHONY: demo_alerts-page-sk
demo_alerts-page-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/alerts-page-sk:demo_page_server

.PHONY: demo_algo-select-sk
demo_algo-select-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/algo-select-sk:demo_page_server

.PHONY: demo_calendar-input-sk
demo_calendar-input-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/calendar-input-sk:demo_page_server

.PHONY: demo_calendar-sk
demo_calendar-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/calendar-sk:demo_page_server

.PHONY: demo_cluster-lastn-page-sk
demo_cluster-lastn-page-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/cluster-lastn-page-sk:demo_page_server

.PHONY: demo_cluster-page-sk
demo_cluster-page-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/cluster-page-sk:demo_page_server

.PHONY: demo_cluster-summary2-sk
demo_cluster-summary2-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/cluster-summary2-sk:demo_page_server

.PHONY: demo_commit-detail-panel-sk
demo_commit-detail-panel-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/commit-detail-panel-sk:demo_page_server

.PHONY: demo_commit-detail-picker-sk
demo_commit-detail-picker-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/commit-detail-picker-sk:demo_page_server

.PHONY: demo_commit-detail-sk
demo_commit-detail-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/commit-detail-sk:demo_page_server

.PHONY: demo_day-range-sk
demo_day-range-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/day-range-sk:demo_page_server

.PHONY: demo_domain-picker-sk
demo_domain-picker-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/domain-picker-sk:demo_page_server

.PHONY: demo_ingest-file-links-sk
demo_ingest-file-links-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/ingest-file-links-sk:demo_page_server

.PHONY: demo_json-source-sk
demo_json-source-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/json-source-sk:demo_page_server

.PHONY: demo_perf-scaffold-sk
demo_perf-scaffold-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/perf-scaffold-sk:demo_page_server

.PHONY: demo_pivot-table-sk
demo_pivot-table-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/pivot-table-sk:demo_page_server

.PHONY: demo_plot-simple-sk
demo_plot-simple-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/plot-simple-sk:demo_page_server

.PHONY: demo_query-chooser-sk
demo_query-chooser-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/query-chooser-sk:demo_page_server

.PHONY: demo_query-count-sk
demo_query-count-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/query-count-sk:demo_page_server

.PHONY: demo_triage-page-sk
demo_triage-page-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/triage-page-sk:demo_page_server

.PHONY: demo_triage-status-sk
demo_triage-status-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/triage-status-sk:demo_page_server

.PHONY: demo_triage2-sk
demo_triage2-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/triage2-sk:demo_page_server

.PHONY: demo_tricon2-sk
demo_tricon2-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/tricon2-sk:demo_page_server

.PHONY: demo_trybot-page-sk
demo_trybot-page-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/trybot-page-sk:demo_page_server

.PHONY: demo_word-cloud-sk
demo_word-cloud-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/word-cloud-sk:demo_page_server

# Presumes you've already run:
#
#    make build
#
#    kubectl port-forward perf-cockroachdb-0 25000:26257
#
# Run "bazel build //perf/..." after making any frontend changes to pick up the
# change without restarting the golang server.
run-local-instance:
	go run ./go/perfserver -- frontend \
		--local \
		--do_clustering=false \
		--port=:8001 \
		--noemail \
		--config_filename=./configs/cdb-nano.json \
		--num_continuous_parallel=1 \
		--display_group_by=true \
		--disable_metrics_update \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:25000/skia?sslmode=disable \
		--commit_range_url=https://skia.googlesource.com/skia/+log/{begin}..{end}


run-flutter-instance:
	go run ./go/perfserver -- frontend \
		--local \
		--do_clustering=false \
		--port=:8001 \
		--noemail \
		--config_filename=./configs/flutter-engine2.json \
		--num_continuous_parallel=1 \
		--display_group_by=true \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:25000/flutter_engine2?sslmode=disable

# Before running this target you need to have a local single-instance of
# cockroachdb running, started like this:
#
#  cd /tmp cockroach start-single-node --insecure --listen-addr=127.0.0.1
#
# Once the local instance of Perf is running you can visit:
#
# 	http://localhost:8002/.
run-demo-instance:
	$(BAZEL) build --config=mayberemote -c dbg //perf/...
	$(BAZEL) run --config=mayberemote -c dbg //perf/go/initdemo:initdemo
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver ingest \
		--local \
		--config_filename=./configs/demo.json \
		--connection_string=postgresql://root@127.0.0.1:26257/demo?sslmode=disable

	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
		--local \
		--do_clustering=false \
		--port=:8002 \
		--prom_port=:20001 \
		--config_filename=./configs/demo.json \
		--display_group_by=false \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:26257/demo?sslmode=disable


# Before running this target you need to have a local single-instance of
# cockroachdb running, started like this:
#
#  cd /tmp cockroach start-single-node --insecure --listen-addr=127.0.0.1
run-demo-maintenance:
	$(BAZEL) build --config=mayberemote -c dbg //perf/...
	$(BAZEL) run --config=mayberemote -c dbg //perf/go/initdemo:initdemo
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver ingest \
		--local \
		--config_filename=./configs/demo.json \
		--connection_string=postgresql://root@127.0.0.1:26257/demo?sslmode=disable
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver maintenance \
		--local \
		--config_filename=./configs/demo.json \
		--connection_string=postgresql://root@127.0.0.1:26257/demo?sslmode=disable \
		--migrate_regressions \
		--refresh_query_cache \
		--tiles_for_query_cache=5

run-demo-backend:
	$(BAZEL) build --config=mayberemote -c dbg //perf/...
	$(BAZEL) run --config=mayberemote -c dbg //perf/go/initdemo:initdemo
	../_bazel_bin/perf/go/backend/backendserver/backendserver_/backendserver run \
		--config_filename=./configs/demo.json \
		--port=:8005 \
		--prom_port=:20002 \
		--commit_range_url=https://chromium.googlesource.com/chromium/src/+log/{begin}..{end}

run-demo-chromeperf:
	$(BAZEL) build --config=mayberemote -c dbg //perf/...
	$(BAZEL) run --config=mayberemote -c dbg //perf/go/initdemo:initdemo
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver ingest \
		--local \
		--config_filename=./configs/demo.json \
		--connection_string=postgresql://root@127.0.0.1:26257/demo?sslmode=disable

	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
		--local \
		--do_clustering=false \
		--port=:8002 \
		--prom_port=:20001 \
		--config_filename=./configs/demo.json \
		--display_group_by=false \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--commit_range_url=https://chromium.googlesource.com/chromium/src/+log/{begin}..{end}
		--fetch_chrome_perf_anomalies=true
		--connection_string=postgresql://root@127.0.0.1:26257/demo?sslmode=disable

run-demo-cluster:
	$(BAZEL) build --config=mayberemote -c dbg //perf/...
	$(BAZEL) run --config=mayberemote -c dbg //perf/go/initdemo:initdemo
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver ingest \
		--local \
		--config_filename=./configs/demo.json \
		--connection_string=postgresql://root@127.0.0.1:26257/demo?sslmode=disable

	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver cluster \
	    --local \
	    --prom_port=:20000 \
        --config_filename=./configs/demo.json \
        --num_continuous=50 \
        --num_paramsets_for_queries=20 \
        --port=:7000 \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
        --num_continuous_parallel=1 \
        --do_clustering=true \
        --event_driven_regression_detection=true

run-auth-proxy-before-demo-instance:
	bazelisk run //kube/cmd/auth-proxy -- \
		--prom-port=:20003 \
		--role=editor=google.com \
		--authtype=mocked \
		--mock_user=$(USER)@google.com \
		--port=:8003 \
		--target_port=http://127.0.0.1:8002 \
		--local

run-auth-proxy-before-backend:
	bazelisk run //kube/cmd/auth-proxy -- \
		--prom-port=:20003 \
		--role=editor=google.com \
		--authtype=mocked \
		--mock_user=$(USER)@google.com \
		--port=:8003 \
		--target_port=:8005 \
		--local \
		--self_sign_localhost_tls

# Used to test chrome-perf-public ingestion into a local CDB instance.
#
# Before running this target you need to have a local single-instance of
# cockroachdb running, started like this:
#
#  cd /tmp cockroach start-single-node --insecure --listen-addr=127.0.0.1
#
run-chrome-ingest-demo-instance:
	$(BAZEL) build --config=mayberemote -c dbg //perf/...
	$(BAZEL) run --config=mayberemote -c dbg //perf/go/initdemo:initdemo -- --databasename=chrome_pub
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver ingest \
		--local \
		--num_parallel_ingesters=1 \
		--config_filename=./configs/chrome-perf-public.json \
		--connection_string=postgresql://root@127.0.0.1:26257/chrome_pub?sslmode=disable
