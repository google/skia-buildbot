# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js \
          res/js/leasing.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

include ../go/skiaversion/skiaversion.mk

default:
	go install -v ./go/...

push: default
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli leasingd skia-leasing
