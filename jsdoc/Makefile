.PHONY: build docs release push
build:
	go install  ./go/jsdoc

docs:
	npm install
	./node_modules/.bin/jsdoc -c jsdoc.config.js

release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/jsdoc
	./build_docker_release

push: release skia-public
	pushk jsdoc

include ../make/clusters.mk
