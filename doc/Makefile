BOWER_DIR=third_party/bower_components

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
  ${BOWER_DIR}/google-code-prettify/bin/prettify.min.js \
	../res/js/common.js


default: core_js res/css/docs.css
	go install -v ./go/docserver

res/css/docs.css: node_modules/lastupdate res/css/main.css ${BOWER_DIR}/google-code-prettify/bin/prettify.min.css
	cat res/css/main.css ${BOWER_DIR}/google-code-prettify/bin/prettify.min.css | ./node_modules/.bin/autoprefixer  -o res/css/docs.css

release: default
	./build_release

include ../webtools/webtools.mk
