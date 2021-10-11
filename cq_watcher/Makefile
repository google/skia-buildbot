include ../make/bazel.mk
include ../kube/kube.mk

default:
	go install -v ./go/...

release: default
	$(BAZEL) run //cq_watcher:push_cq_watcher_container

push: pushk release
	pushk --use-temp-checkout cq_watcher
