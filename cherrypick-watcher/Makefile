include ../make/bazel.mk

default: build

build:
	go install -v ./go/cherrypick-watcher

release:
	$(BAZEL) run //cherrypick-watcher:push_cherrypick_watcher_container

push: release
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout cherrypick-watcher

testgo:
	go test ./go/...

run-local-instance: build
	go run go/cherrypick-watcher/main.go --local
