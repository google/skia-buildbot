# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_components via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
					third_party/bower_components/webcomponentsjs/webcomponents-lite.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

.PHONY: build
build: skiaperf web ptracequery ingest_json_validator

build-perf-ingest:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/perf-ingest
	./build_perf_ingest_release

push-perf-ingest: build-perf-ingest
	pushk perf-ingest

.PHONY: validate
validate: ingest_json_validator
	ingest_json_validator --config_filename=sys/android-master-perf.json5
	ingest_json_validator --config_filename=sys/android-perf.json5
	ingest_json_validator --config_filename=sys/perf.json5
	ingest_json_validator --config_filename=default.json5

.PHONY: skiaperf
skiaperf:
	go install -v ./go/skiaperf

ptracequery:
	go install -v ./go/ptracequery

ingest_json_validator:
	go install -v ./go/ingest_json_validator

.PHONY: web
web: clean_webtools elements_html core_js

.PHONY: release
release: build
	echo

.PHONY: test
test: testgo
	echo

.PHONY: testgo
testgo:
	go test -v ./go/...

.PHONY: push
push: build
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli skiaperfd skia-perf

.PHONY: push_android
push_android: build
	./build_android_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli android-skiaperfd skia-android-perf

.PHONY: push_android_master
push_android_master: build
	./build_master_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli android-master-skiaperfd skia-android-master-perf

.PHONY: perf_update_regressions
perf_update_regressions:
	go install -v ./go/perf_update_regressions

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
