include ../make/bazel.mk

default: build

build:
	go install -v ./go/cherrypick-watcher

release:
	$(BAZEL) run --config=mayberemote //cherrypick-watcher:push_cherrypick_watcher_container

testgo:
	go test ./go/...

run-local-instance: build
	go run go/cherrypick-watcher/main.go --local
