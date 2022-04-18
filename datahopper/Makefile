include ../make/bazel.mk

.PHONY: build
build: build
	$(BAZEL) build //datahopper/...

.PHONY: release
release: build
	$(BAZEL) run //datahopper:push_datahopper_container

.PHONY: push
push: release
	$(BAZEL) run //kube/pushk -- --message="$(MESSAGE)" --use-temp-checkout datahopper
