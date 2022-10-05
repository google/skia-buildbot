include ../make/bazel.mk

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //static_server:push_static_server

.PHONY: push
push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- static_server