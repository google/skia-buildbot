# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_components via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
  third_party/bower_components/webcomponentsjs/webcomponents-lite.min.js \
  ../res/js/common.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

.PHONY: build
build: elements_html core_js
	go install -v ./go/imageinfo

.PHONY: testgo
testgo:
	go test ./...

.PHONY: push
push: build
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli imageinfo skia-imageinfo

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

include ../webtools/webtools.mk
