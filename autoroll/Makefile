VULCANIZE1=true

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
          third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

include ../go/skiaversion/skiaversion.mk

all: autoroll-fe autoroll-be autoroll-google3

autoroll-fe-kube: core_js elements_html skiaversion
	CGO_ENABLED=0 GOOS=linux go install -a ./go/autoroll-fe-kube

autoroll-be: skiaversion
	go install -v ./go/autoroll-be

autoroll-google3: skiaversion
	go install -v ./go/autoroll-google3

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

testgo: skiaversion
	go test ./go/... -v

include ../webtools/webtools.mk

release-fe-kube: autoroll-fe-kube
	./build_docker_frontend_release

ROLLER_CONFIGS=$(wildcard config/*.json)

release-be: autoroll-be
	./build_backend_release "$(CONFIG_FILE)" "$(MESSAGE)"

release-be-all: autoroll-be $(ROLLER_CONFIGS)
	$(foreach cfg, $(ROLLER_CONFIGS), ./build_backend_release "$(cfg)" "$(MESSAGE)";)

release-google3: autoroll-google3
	./build_google3_release "$(MESSAGE)"

release-all: release-fe-kube release-be-all release-google3
