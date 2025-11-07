include ../make/bazel.mk

default:
	go install -v ./go/...

release: default
	$(BAZEL) run --config=mayberemote //cq_watcher:push_cq_watcher_container
