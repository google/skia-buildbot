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

.PHONY: leasing_release
leasing_release: leasing
	./build_release "`git log -n1 --format=%s`"

.PHONY: leasing_push
leasing_push: leasing
	go install -v ../push/go/pushcli
	./build_release "`git log -n1 --format=%s`"
	pushcli leasingd skia-leasing
