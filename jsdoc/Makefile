.PHONY: build docs release push
build:
	go install  ./go/jsdoc

docs:
	npm install
	./node_modules/.bin/jsdoc -c jsdoc.config.js

release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/jsdoc
	-rm -rf ./dist
	mkdir ./dist
	cd ./.dist
	git clone https://github.com/google/elements-sk.git
	git clone https://skia.googlesource.com/buildbot/
	cd ./elements-sk
	make release
	rm -rf node_modules
	cd ../buildbot/jsdoc
	make docs
	rm -rf node_modules
	cd ../common-sk
	make demos
	rm -rf node_modules
	./build_docker_release

push: release skia-public
	pushk jsdoc

include ../make/clusters.mk
