include ../make/bazel.mk

.PHONY: release
release:
	$(BAZEL) run //static_server:push_static_server

.PHONY: push
push: release
	$(BAZEL) run //kube/go/pushk -- static_server