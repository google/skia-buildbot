include ../make/bazel.mk

.PHONY: build
build: ./modules/json/index.ts
	$(BAZEL) build //perf/... --config=remote

.PHONY: docs
docs:
	$(BAZEL) run //perf/go/perf-tool:perf-tool -- markdown > PERFTOOL.md
	$(BAZEL) run //perf/go/perfserver --  markdown > PERFSERVER.md

.PHONY: release
release:
	$(BAZEL) run //perf:push_perfserver

.PHONY: push
push:
	$(BAZEL) run //perf:pushk_perfserver

.PHONY: release_backup
release_backup:
	$(BAZEL) run //perf:push_perf-cockroachdb-backup

.PHONY: push_backup
push_backup: release_backup
	$(BAZEL) run //perf:pushk_perf-cockroachdb-backup

# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/perf/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles)
	go generate ./go/ts

.PHONY: test
test:
	$(BAZEL) test //perf/... --config=remote

.PHONY: demo_alert-config-sk
demo_alert-config-sk:
	$(BAZEL) run //perf/modules/alert-config-sk:demo_page_server

.PHONY: demo_alerts-page-sk
demo_alerts-page-sk:
	$(BAZEL) run //perf/modules/alerts-page-sk:demo_page_server

.PHONY: demo_algo-select-sk
demo_algo-select-sk:
	$(BAZEL) run //perf/modules/algo-select-sk:demo_page_server

.PHONY: demo_calendar-input-sk
demo_calendar-input-sk:
	$(BAZEL) run //perf/modules/calendar-input-sk:demo_page_server

.PHONY: demo_calendar-sk
demo_calendar-sk:
	$(BAZEL) run //perf/modules/calendar-sk:demo_page_server

.PHONY: demo_cluster-lastn-page-sk
demo_cluster-lastn-page-sk:
	$(BAZEL) run //perf/modules/cluster-lastn-page-sk:demo_page_server

.PHONY: demo_cluster-page-sk
demo_cluster-page-sk:
	$(BAZEL) run //perf/modules/cluster-page-sk:demo_page_server

.PHONY: demo_cluster-summary2-sk
demo_cluster-summary2-sk:
	$(BAZEL) run //perf/modules/cluster-summary2-sk:demo_page_server

.PHONY: demo_commit-detail-panel-sk
demo_commit-detail-panel-sk:
	$(BAZEL) run //perf/modules/commit-detail-panel-sk:demo_page_server

.PHONY: demo_commit-detail-picker-sk
demo_commit-detail-picker-sk:
	$(BAZEL) run //perf/modules/commit-detail-picker-sk:demo_page_server

.PHONY: demo_commit-detail-sk
demo_commit-detail-sk:
	$(BAZEL) run //perf/modules/commit-detail-sk:demo_page_server

.PHONY: demo_day-range-sk
demo_day-range-sk:
	$(BAZEL) run //perf/modules/day-range-sk:demo_page_server

.PHONY: demo_domain-picker-sk
demo_domain-picker-sk:
	$(BAZEL) run //perf/modules/domain-picker-sk:demo_page_server

.PHONY: demo_ingest-file-links-sk
demo_ingest-file-links-sk:
	$(BAZEL) run //perf/modules/ingest-file-links-sk:demo_page_server

.PHONY: demo_json-source-sk
demo_json-source-sk:
	$(BAZEL) run //perf/modules/json-source-sk:demo_page_server

.PHONY: demo_perf-scaffold-sk
demo_perf-scaffold-sk:
	$(BAZEL) run //perf/modules/perf-scaffold-sk:demo_page_server

.PHONY: demo_pivot-table-sk
demo_pivot-table-sk:
	$(BAZEL) run //perf/modules/pivot-table-sk:demo_page_server

.PHONY: demo_plot-simple-sk
demo_plot-simple-sk:
	$(BAZEL) run //perf/modules/plot-simple-sk:demo_page_server

.PHONY: demo_query-chooser-sk
demo_query-chooser-sk:
	$(BAZEL) run //perf/modules/query-chooser-sk:demo_page_server

.PHONY: demo_query-count-sk
demo_query-count-sk:
	$(BAZEL) run //perf/modules/query-count-sk:demo_page_server

.PHONY: demo_triage-page-sk
demo_triage-page-sk:
	$(BAZEL) run //perf/modules/triage-page-sk:demo_page_server

.PHONY: demo_triage-status-sk
demo_triage-status-sk:
	$(BAZEL) run //perf/modules/triage-status-sk:demo_page_server

.PHONY: demo_triage2-sk
demo_triage2-sk:
	$(BAZEL) run //perf/modules/triage2-sk:demo_page_server

.PHONY: demo_tricon2-sk
demo_tricon2-sk:
	$(BAZEL) run //perf/modules/tricon2-sk:demo_page_server

.PHONY: demo_trybot-page-sk
demo_trybot-page-sk:
	$(BAZEL) run //perf/modules/trybot-page-sk:demo_page_server

.PHONY: demo_word-cloud-sk
demo_word-cloud-sk:
	$(BAZEL) run //perf/modules/word-cloud-sk:demo_page_server

# Presumes you've already run:
#
#    make build
#
#    kubectl port-forward perf-cockroachdb-0 25000:26257
#
# Run "bazel build //perf/..." after making any frontend changes to pick up the
# change without restarting the golang server.
run-local-instance:
	$(BAZEL) build //perf/...
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
		--local \
		--do_clustering=false \
		--port=:8001 \
		--noemail \
		--config_filename=./configs/cdb-nano.json \
		--num_continuous_parallel=1 \
		--display_group_by=true \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:25000/skia?sslmode=disable


run-demo-instance:
	$(BAZEL) build //perf/...
	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver ingest \
		--local \
		--config_filename=./configs/demo.json \
		--connection_string=postgresql://root@127.0.0.1:25000/demo?sslmode=disable

	../_bazel_bin/perf/go/perfserver/perfserver_/perfserver frontend \
		--local \
		--do_clustering=false \
		--port=:8001 \
		--noemail \
		--config_filename=./configs/demo.json \
		--num_continuous_parallel=1 \
		--display_group_by=false \
		--resources_dir=../_bazel_bin/perf/pages/development/ \
		--connection_string=postgresql://root@127.0.0.1:25000/demo?sslmode=disable
