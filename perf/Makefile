# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_components via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
					third_party/bower_components/webcomponentsjs/webcomponents-lite.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

.PHONY: build
build: skiaperf web ingest_json_validator

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

.PHONY: skiaperf
skiaperf:
	go install -v ./go/skiaperf

ingest_json_validator:
	go install -v ./go/ingest_json_validator

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
