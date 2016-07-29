VULCANIZE1=true

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components


include ../go/skiaversion/skiaversion.mk

all: autoroll

autoroll: core_js elements_html skiaversion
	ln -sf ../../res res/imp/res	
	go install -v ./go/autoroll

testgo: skiaversion
	go test ./go/... -v

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

include ../webtools/webtools.mk

skia_release: autoroll
	./build_skia_release "$(MESSAGE)"

nacl_release: autoroll
	./build_nacl_release "$(MESSAGE)"

catapult_release: autoroll
	./build_catapult_release "$(MESSAGE)"
