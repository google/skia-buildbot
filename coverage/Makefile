#Use Vulcanize 1.0
VULCANIZE1=true
# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

default: all

.PHONY: all
all: clean_webtools core_js elements_html skiaversion coverage_server

.PHONY: coverage_server
coverage_server:
	go install -v ./go/coverage-server

.PHONY: release
release: all
	./coverage_release "`git log -n1 --format=%s`"

include ../webtools/webtools.mk
include ../go/skiaversion/skiaversion.mk
