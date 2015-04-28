# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

.PHONY: all
all: res/css/fuzzer.css core_js elements_html frontend fuzzer

.PHONY: ver
ver:    H=$(shell echo `git log -n 1 --format=format:%H`)
ver:
	S=$(shell echo $H | cut -c1-5); \
	sed --in-place "s/\(<span id=\"version\">\)\(.*\)\(<\/span>\)/\1<a href=\"https:\/\/skia.googlesource.com\/buildbot\/+\/$H\" target=\"_blank\">Ver:$$S<\/a>\3/" templates/titlebar.html
	git update-index --assume-unchanged templates/titlebar.html

.PHONY: release
release: ver all
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

.PHONY: frontend
frontend:
	go install -v ./go/frontend

.PHONY: aggregator
aggregator:
	go install -v ./go/aggregator

.PHONY: sanitizer
sanitizer:
	go install -v ./go/sanitizer

.PHONY: fuzzer
fuzzer:
	go install -v ./go/fuzzer

# Build serving CSS from input CSS.
res/css/fuzzer.css: node_modules/lastupdate res/css/main.css
	./node_modules/.bin/autoprefixer res/css/main.css -o res/css/fuzzer.css

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append

include ../webtools/webtools.mk
