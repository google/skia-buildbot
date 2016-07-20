#Use Vulcanize 1.0
VULCANIZE1=true
# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js \
          res/js/fuzzer.js

BOWER_DIR=third_party/bower_components

default: all

all: clean_webtools core_js elements_html skiaversion frontend backend update

release: all
	./build_release "$(MESSAGE)"

test: testgo testjs
	echo

testgo:
	go test ./go/...

frontend:
	go install -v ./go/fuzzer-fe

backend:
	go install -v ./go/fuzzer-be

update:
	go install -v ./go/update_version_to_fuzz


include ../webtools/webtools.mk
include ../go/skiaversion/skiaversion.mk
