include ../make/bazel.mk
include ../kube/kube.mk

default: build

build:
	go install -v ./go/codereview-watcher

release:
	$(BAZEL) run //codereview-watcher:push_codereview_watcher_container

push: pushk release
	pushk --use-temp-checkout codereview-watcher

testgo:
	go test ./go/...
