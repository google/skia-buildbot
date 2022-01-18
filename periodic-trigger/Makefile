include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: build
build:
	$(BAZEL) build //periodic-trigger/go/...

.PHONY: release
release: build
	$(BAZEL) run //periodic-trigger:push_periodic_trigger_container

.PHONY: push
push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout periodic-trigger
