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

release: autoroll android_master_release android_o_release angle_skia_release angle_chromium_release catapult_release depot_tools_chromium_release google3_release ios_internal_chromium_release nacl_release pdfium_release fuchsia_release skcms_skia_release skia_release skia_internal_release src_internal_chromium_release webrtc_chromium_release

android_master_release: autoroll
	./build_android_master_release "$(MESSAGE)"

android_o_release: autoroll
	./build_android_o_release "$(MESSAGE)"

angle_skia_release: autoroll
	./build_angle_skia_release "$(MESSAGE)"

angle_chromium_release: autoroll
	./build_angle_chromium_release "$(MESSAGE)"

catapult_release: autoroll
	./build_catapult_release "$(MESSAGE)"

depot_tools_chromium_release: autoroll
	./build_depot_tools_chromium_release "$(MESSAGE)"

.PHONY: google3_release
google3_release: autoroll
	./build_google3_release "$(MESSAGE)"

ios_internal_chromium_release: autoroll
	./build_ios_internal_chromium_release "$(MESSAGE)"

nacl_release: autoroll
	./build_nacl_release "$(MESSAGE)"

pdfium_release: autoroll
	./build_pdfium_release "$(MESSAGE)"

fuchsia_release: autoroll
	./build_fuchsia_release "$(MESSAGE)"

skcms_skia_release: autoroll
	./build_skcms_skia_release "$(MESSAGE)"

skia_release: autoroll
	./build_skia_release "$(MESSAGE)"

skia_internal_release: autoroll
	./build_skia_internal_release "$(MESSAGE)"

src_internal_chromium_release: autoroll
	./build_src_internal_chromium_release "$(MESSAGE)"

webrtc_chromium_release: autoroll
	./build_webrtc_chromium_release "$(MESSAGE)"
