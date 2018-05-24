.PHONY: build release push
build:
	go install  ./go/jsdoc

release:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/jsdoc
	./build_docker_release

push:
	pushk jsdoc
