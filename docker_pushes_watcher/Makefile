include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: build test pushk release push

build:
	go install  ./go/...

test:
	go test ./go/...

release:
	$(BAZEL) run //docker_pushes_watcher:push_docker_pushes_watcher_container

push: pushk release
	pushk --use-temp-checkout docker_pushes_watcher
