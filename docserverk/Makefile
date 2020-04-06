BOWER_DIR=third_party/bower_components

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents-lite.min.js \
  ${BOWER_DIR}/google-code-prettify/bin/prettify.min.js \
	../res/js/common.js \
	./res/js/docs.js

default: core_js res/css/docs.css
	go install -v ./go/docserverk

res/css/docs.css: node_modules/lastupdate res/css/main.css ${BOWER_DIR}/google-code-prettify/bin/prettify.min.css
	cat res/css/main.css ${BOWER_DIR}/google-code-prettify/bin/prettify.min.css > res/css/docs.css

.PHONY: release
release: default
	CGO_ENABLED=0 GOOS=linux go install -a ./go/docserverk
	./build_docker_release

.PHONY: push
push: release
	go install ../kube/go/pushk
	pushk docserverk

.PHONY: run
run:
	# Runs the latest built release.
	docker run -ti -p8000:8000 docserverk --resources_dir=/usr/local/share/docserverk/ --alsologtostderr --local

include ../webtools/webtools.mk
