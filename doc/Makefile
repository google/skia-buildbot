BOWER_DIR=third_party/bower_components

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
  third_party/bower_components/webcomponentsjs/webcomponents-lite.min.js \
  ${BOWER_DIR}/google-code-prettify/bin/prettify.min.js \
	../res/js/common.js \
	./res/js/docs.js

default: core_js res/css/docs.css
	go install -v ./go/docserver

res/css/docs.css: node_modules/lastupdate res/css/main.css ${BOWER_DIR}/google-code-prettify/bin/prettify.min.css
	cat res/css/main.css ${BOWER_DIR}/google-code-prettify/bin/prettify.min.css | ./node_modules/.bin/autoprefixer  -o res/css/docs.css

release: default
	./build_release

.PHONY: push
push: default
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli docserver skia-docs

include ../webtools/webtools.mk

NOW=`date --iso-8601=seconds`

docker_release:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./docker/docserver ./go/docserver

docker:
	cp ./templates/index.html ./docker
	cp ./res/js/core.js       ./docker
	cp ./res/img/logo.png     ./docker
	cp ./res/css/docs.css     ./docker
	docker build -t docserver ./docker

push_docker:
	docker tag docserver gcr.io/skia-public/docserver:2.1
	docker push gcr.io/skia-public/docserver:2.1

.PHONY: docker

