include ../make/bazel.mk

.PHONY: build
build: ./modules/json/index.ts
	$(BAZEL) build --config=mayberemote //perf/...

generate_sql_schema:
	cd ./go/sql; go run ./tosql
	go run ./go/sql/exportschema --out ./go/sql/expectedschema/schema_spanner.json \
	--databaseType spanner

.PHONY: docs
docs:
	$(BAZEL) run --config=mayberemote //perf/go/perf-tool:perf-tool -- markdown > PERFTOOL.md
	$(BAZEL) run --config=mayberemote //perf/go/perfserver --  markdown > PERFSERVER.md

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //perf:push_perfserver

# Run a spanner emulator locally.
run-spanner-emulator:
	sudo docker ps -q | xargs -r sudo docker rm -vf
	sudo docker run -d -p 5432:5432 gcr.io/cloud-spanner-pg-adapter/pgadapter-emulator

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

.PHONY: demo_json-source-sk
demo_json-source-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/json-source-sk:demo_page_server

.PHONY: demo_report-page-sk
demo_report-page-sk:
	$(BAZEL) run --config=mayberemote //perf/modules/report-page-sk:demo_page_server

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

initdemo:
	$(BAZEL) build --config=mayberemote -c dbg //perf/...
	$(BAZEL) run --config=mayberemote -c dbg //perf/go/initdemo:initdemo -- \
		--database_url=postgresql://localhost:5432/demo?sslmode=disable

ingest_demo:
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver ingest \
		--local \
		--config_filename=./configs/demo_spanner.json \
		--connection_string=postgresql://root@127.0.0.1:5432/demo?sslmode=disable

run_frontend:
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
		--dev_mode \
		--do_clustering=false \
		--port=:8002 \
		--prom_port=:20001 \
		--config_filename=./configs/demo_spanner.json \
		--display_group_by=false \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:5432/demo?sslmode=disable

run_frontend_with_clustering:
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
		--dev_mode \
		--do_clustering=true \
		--port=:8002 \
		--prom_port=:20001 \
		--config_filename=./configs/demo_spanner.json \
		--display_group_by=false \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:5432/demo?sslmode=disable

run-demo-instance: run-spanner-emulator initdemo ingest_demo run_frontend
run-demo-instance-db-persist: initdemo run_frontend

run_maintenance:
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver maintenance \
		--dev_mode \
		--config_filename=./configs/demo_spanner.json \
		--connection_string=postgresql://root@127.0.0.1:5432/demo?sslmode=disable \
		--migrate_regressions \
		--refresh_query_cache \
		--tiles_for_query_cache=5

run-demo-maintenance: run-spanner-emulator initdemo ingest_demo run_maintenance

run-demo-backend:
	$(BAZEL) build --config=mayberemote -c dbg //perf/...
	$(BAZEL) run --config=mayberemote -c dbg //perf/go/initdemo:initdemo
	../_bazel_bin/perf/go/backend/backendserver/backendserver_/backendserver run \
		--config_filename=./configs/demo.json \
		--port=:8005 \
		--prom_port=:20002 \
		--commit_range_url=https://chromium.googlesource.com/chromium/src/+log/{begin}..{end}

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

insert-demo-alert-to-demo-db:
	./scripts/add_demo_alert_to_demo_db.sh

.PHONY: run-demo-instance-with-clustering
run-demo-instance-with-clustering:
	@echo "starting frontend with clustering and a sample alert"
	$(MAKE) run-spanner-emulator initdemo ingest_demo \
	insert-demo-alert-to-demo-db run_frontend_with_clustering

