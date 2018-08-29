.PHONY: build release push
build:
	go install  ./go/...

release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/notifier
	./build_docker_release

push: release skia-public
	pushk notifier

include ../make/clusters.mk
