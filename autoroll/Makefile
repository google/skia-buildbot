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

release: autoroll afdo_chromium_release android_master_release android_next_release android_o_release angle_skia_release angle_chromium_release catapult_release chromite_chromium_release chromium_skia_release depot_tools_chromium_release flutter_engine_flutter_release fuchsia_sdk_chromium_release google3_release ios_internal_chromium_release lottie_web_lottie_ci_release nacl_release pdfium_release perfetto_chromium_release fuchsia_release skcms_skia_release skia_release skia_flutter_release skia_internal_release skia_lottie_ci_release src_internal_chromium_release swiftshader_skia_release webrtc_chromium_release

afdo_chromium_release: autoroll
	./build_afdo_chromium_release "$(MESSAGE)"

android_master_release: autoroll
	./build_android_master_release "$(MESSAGE)"

android_next_release: autoroll
	./build_android_next_release "$(MESSAGE)"

android_o_release: autoroll
	./build_android_o_release "$(MESSAGE)"

angle_skia_release: autoroll
	./build_angle_skia_release "$(MESSAGE)"

angle_chromium_release: autoroll
	./build_angle_chromium_release "$(MESSAGE)"

catapult_release: autoroll
	./build_catapult_release "$(MESSAGE)"

chromite_chromium_release: autoroll
	./build_chromite_chromium_release "$(MESSAGE)"

chromium_skia_release: autoroll
	./build_chromium_skia_release "$(MESSAGE)"

depot_tools_chromium_release: autoroll
	./build_depot_tools_chromium_release "$(MESSAGE)"

flutter_engine_flutter_release: autoroll
	./build_flutter_engine_flutter_release "$(MESSAGE)"

fuchsia_sdk_chromium_release: autoroll
	./build_fuchsia_sdk_chromium_release "$(MESSAGE)"

.PHONY: google3_release
google3_release: autoroll
	./build_google3_release "$(MESSAGE)"

ios_internal_chromium_release: autoroll
	./build_ios_internal_chromium_release "$(MESSAGE)"

lottie_web_lottie_ci_release: autoroll
	./build_lottie_web_lottie_ci_release "$(MESSAGE)"

nacl_release: autoroll
	./build_nacl_release "$(MESSAGE)"

pdfium_release: autoroll
	./build_pdfium_release "$(MESSAGE)"

perfetto_chromium_release: autoroll
	./build_perfetto_chromium_release "$(MESSAGE)"

fuchsia_release: autoroll
	./build_fuchsia_release "$(MESSAGE)"

skcms_skia_release: autoroll
	./build_skcms_skia_release "$(MESSAGE)"

skia_release: autoroll
	./build_skia_release "$(MESSAGE)"

skia_flutter_release: autoroll
	./build_skia_flutter_release "$(MESSAGE)"

skia_internal_release: autoroll
	./build_skia_internal_release "$(MESSAGE)"

skia_lottie_ci_release: autoroll
	./build_skia_lottie_ci_release "$(MESSAGE)"

src_internal_chromium_release: autoroll
	./build_src_internal_chromium_release "$(MESSAGE)"

swiftshader_skia_release: autoroll
	./build_swiftshader_skia_release "$(MESSAGE)"

webrtc_chromium_release: autoroll
	./build_webrtc_chromium_release "$(MESSAGE)"
