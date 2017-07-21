VULCANIZE1=true

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components


include ../go/depot_tools/depot_tools.mk
include ../go/skiaversion/skiaversion.mk

all: autoroll

autoroll: core_js depot_tools elements_html skiaversion
	ln -sf ../../res res/imp/res
	go install -v ./go/autoroll

testgo: depot_tools skiaversion
	go test ./go/... -v

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

include ../webtools/webtools.mk

release: autoroll android_master_release android_o_release angle_release catapult_release nacl_release pdfium_release fuchsia_release skia_release skia_internal_release

android_master_release: autoroll
	./build_android_master_release "$(MESSAGE)"

android_o_release: autoroll
	./build_android_o_release "$(MESSAGE)"

angle_release: autoroll
	./build_angle_release "$(MESSAGE)"

catapult_release: autoroll
	./build_catapult_release "$(MESSAGE)"

nacl_release: autoroll
	./build_nacl_release "$(MESSAGE)"

pdfium_release: autoroll
	./build_pdfium_release "$(MESSAGE)"

fuchsia_release: autoroll
	./build_fuchsia_release "$(MESSAGE)"

skia_release: autoroll
	./build_skia_release "$(MESSAGE)"

skia_internal_release: autoroll
	./build_skia_internal_release "$(MESSAGE)"
