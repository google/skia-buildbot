# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
	third_party/bower_components/webcomponentsjs/webcomponents.min.js \
	../res/js/common.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

build: core_js elements_html
	go install -v ./go/push
	go install -v ./go/pushcli
	go install -v ./go/build_skiapush_json5
	build_skiapush_json5

release: build
	./build_pushd_release "$(MESSAGE)"

push: build
	./build_pushd_release "`git log -n1 --format=%s`"
	pushcli pushd skia-push

debug: clean_webtools debug_core_js debug_elements_html

.PHONY: build release push debug

include ../webtools/webtools.mk
