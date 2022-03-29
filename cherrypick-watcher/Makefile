include ../make/bazel.mk
include ../kube/kube.mk

default: build

build:
	go install -v ./go/cherrypick-watcher

release:
	$(BAZEL) run //cherrypick-watcher:push_cherrypick_watcher_container

push: pushk release
	pushk --use-temp-checkout cherrypick-watcher

testgo:
	go test ./go/...

run-local-instance: build
	go run go/cherrypick-watcher/main.go --local
