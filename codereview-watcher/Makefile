include ../make/bazel.mk

default: build

build:
	go install -v ./go/codereview-watcher

release:
	$(BAZEL) run --config=mayberemote //codereview-watcher:push_codereview_watcher_container

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout codereview-watcher

testgo:
	go test ./go/...
