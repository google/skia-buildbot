include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build --config=mayberemote //periodic-trigger/go/...

.PHONY: release
release: build
	$(BAZEL) run --config=mayberemote //periodic-trigger:push_periodic_trigger_container
