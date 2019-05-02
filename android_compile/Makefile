# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js \

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

include ../webtools/webtools.mk
include ../go/skiaversion/skiaversion.mk

# Build debug versions of core.js and elements.html.
.PHONY: debug_android_compile
debug_android_compile: clean_webtools debug_core_js debug_elements_html skiaversion
	go install -v ./go/...

.PHONY: android_compile
android_compile: clean_webtools elements_html
	GOOS=linux go install -v ./go/...

.PHONY: release
release: android_compile
	./build_docker_release

.PHONY: push
push: skia-corp
	pushk --cluster=skia-corp android_compile

include ../make/clusters.mk
