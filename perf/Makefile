# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js \
          third_party/bower_components/jquery/dist/jquery.min.js \
          third_party/bower_components/flot/jquery.flot.js \
          third_party/bower_components/flot/jquery.flot.crosshair.js \
          third_party/bower_components/flot/jquery.flot.navigate.js  \
          third_party/bower_components/flot/jquery.flot.selection.js

BOWER_DIR=third_party/bower_components

.PHONY: build
build: res/css/perf.css core_js elements_html
	go install -v ./go/skiaperf

.PHONY: ver
ver:    H=$(shell echo `git log -n 1 --format=format:%H`)
ver:
	S=$(shell echo $H | cut -c1-5); \
	sed --in-place "s/\(<span id=\"version\">\)\(.*\)\(<\/span>\)/\1<a href=\"https:\/\/skia.googlesource.com\/buildbot\/+\/$H\" target=\"_blank\">Ver:$$S<\/a>\3/" templates/titlebar.html
	git update-index --assume-unchanged templates/titlebar.html

.PHONY: release
release: ver build
	echo

.PHONY: test
test: testgo testjs
	echo

.PHONY: testgo
testgo:
	go test ./go/...

.PHONY: testjs
testjs:
	./node_modules/karma/bin/karma start karma.conf.js

.PHONY: ingest
ingest:
	go install -v ./go/ingest

.PHONY: logs
logs:
	go install -v ./go/logserver

.PHONY: tool
tool:
	go install -v ./go/tiletool

.PHONY: perf_migratedb
perf_migratedb:
	go install -v ./go/perf_migratedb

# Build serving CSS from input CSS.
res/css/perf.css: node_modules/lastupdate res/css/main.css
	./node_modules/.bin/autoprefixer res/css/main.css -o res/css/perf.css

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

.PHONY: all
all: build ingest tool logs perf_migratedb

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append

include ../webtools/webtools.mk
