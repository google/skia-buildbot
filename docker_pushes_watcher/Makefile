include ../make/bazel.mk

.PHONY: build test pushk release push

build:
	go install  ./go/...

test:
	go test ./go/...

release:
	$(BAZEL) run //docker_pushes_watcher:push_docker_pushes_watcher_container

push: release
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout docker_pushes_watcher
