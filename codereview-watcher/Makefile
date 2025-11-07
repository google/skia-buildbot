include ../make/bazel.mk

default: build

build:
	go install -v ./go/codereview-watcher

release:
	$(BAZEL) run --config=mayberemote //codereview-watcher:push_codereview_watcher_container

testgo:
	go test ./go/...
