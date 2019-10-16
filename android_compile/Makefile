# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js \

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

include ../webtools/webtools.mk
include ../go/skiaversion/skiaversion.mk

# Build debug versions of core.js and elements.html.
.PHONY: debug_android_compile_fe
debug_android_compile_fe: clean_webtools debug_core_js debug_elements_html skiaversion
	go install -v ./go/android_compile_fe/...

.PHONY: android_compile_fe
android_compile_fe: clean_webtools elements_html
	CGO_ENABLED=0 GOOS=linux go install -a ./go/android_compile_fe

.PHONY: release_fe
release_fe: android_compile_fe
	./build_fe_release

.PHONY: android_compile_be
android_compile_be:
	GOOS=linux go install -v ./go/android_compile_be

.PHONY: release_be
release_be: android_compile_be
	./build_be_release

.PHONY: push_fe
push: skia-corp
	pushk --cluster=skia-corp android_compile_fe

.PHONY: push_be
push: skia-corp
	pushk --cluster=skia-corp android_compile_be

include ../make/clusters.mk
