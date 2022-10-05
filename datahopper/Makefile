include ../make/bazel.mk

.PHONY: build
build: build
	$(BAZEL) build --config=mayberemote //datahopper/...

.PHONY: release
release: build
	$(BAZEL) run --config=mayberemote //datahopper:push_datahopper_container

.PHONY: push
push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout datahopper
