.PHONY: build release push
build:
	go install  ./go/...

release:
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/notifier
	./build_docker_release

push:
	pushk notifier
