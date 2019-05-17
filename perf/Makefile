# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_components via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/chart.js/dist/Chart.bundle.min.js \
										node_modules/chartjs-plugin-annotation/chartjs-plugin-annotation.min.js \
										node_modules/chartjs-plugin-zoom/chartjs-plugin-zoom.min.js	 \
										third_party/bower_components/webcomponentsjs/webcomponents-lite.min.js \
										../res/js/common.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

.PHONY: build
build: web ingest_json_validator
	go install -v ./go/...

buildk: web
	CGO_ENABLED=0 GOOS=linux go install -a ./go/skiaperf
	./build_docker_release

build-perf-ingest:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/perf-ingest
	./build_perf_ingest_release

push-perf-ingest: build-perf-ingest skia-public
	pushk perf-ingest

pushk: buildk skia-public
	pushk skiaperf

.PHONY: perf-force-ingest
	go install -v ./go/perf-force-ingest

ingest_json_validator:
	go install -v ./go/ingest_json_validator

start_datastore_emulator:
	 docker run -ti -p 8008:8008 google/cloud-sdk:latest gcloud beta emulators datastore start --no-store-on-disk --project test-project --host-port 0.0.0.0:8008

.PHONY: web
web: clean_webtools elements_html core_js

.PHONY: test
test: testgo
	echo

.PHONY: testgo
testgo:
	go test ./go/...

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

.PHONY: all
all: build

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append

include ../webtools/webtools.mk
include ../make/clusters.mk
