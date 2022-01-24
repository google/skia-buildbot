include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: build
build: build
	$(BAZEL) build //datahopper/...

.PHONY: release
release: build
	$(BAZEL) run //datahopper:push_datahopper_container

.PHONY: push
push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout datahopper
