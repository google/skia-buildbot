#Use Vulcanize 1.0
VULCANIZE1=true
# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js

BOWER_DIR=third_party/bower_components

default: all

all: clean_webtools core_js elements_html skiaversion power

power:
	go install -v ./go/power-controller

power_release: all
	./power_release "`git log -n1 --format=%s`"

include ../webtools/webtools.mk
include ../go/skiaversion/skiaversion.mk
