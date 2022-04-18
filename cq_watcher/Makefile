include ../make/bazel.mk

default:
	go install -v ./go/...

release: default
	$(BAZEL) run //cq_watcher:push_cq_watcher_container

push: release
	$(BAZEL) run //kube/pushk -- --use-temp-checkout cq_watcher
