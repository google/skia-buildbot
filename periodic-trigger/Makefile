include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build --config=mayberemote //periodic-trigger/go/...

.PHONY: release
release: build
	$(BAZEL) run --config=mayberemote //periodic-trigger:push_periodic_trigger_container

.PHONY: push
push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout periodic-trigger
