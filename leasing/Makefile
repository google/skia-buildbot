# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js \
          res/js/leasing.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

include ../webtools/webtools.mk
include ../go/skiaversion/skiaversion.mk

# Build debug versions of core.js and elements.html.
.PHONY: debug_leasing
debug_leasing: clean_webtools debug_core_js debug_elements_html skiaversion
	go install -v ./go/...

.PHONY: leasing
leasing: clean_webtools elements_html
	go install -v ./go/...

.PHONY: release
release: leasing
	CGO_ENABLED=0 GOOS=linux go install -a ./go/leasing
	./build_docker_release

.PHONY: push
push: release skia-public
	pushk leasing

include ../make/clusters.mk
