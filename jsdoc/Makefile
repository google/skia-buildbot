.PHONY: build docs release push
build:
	go install  ./go/jsdoc

docs:
		npx jsdoc -c jsdoc.config.js

release:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/jsdoc
	./build_docker_release

push:
	pushk --apply jsdoc
