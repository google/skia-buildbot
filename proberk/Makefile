include ../make/bazel.mk

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //proberk:push_proberk-base

.PHONY: push
push:
	$(BAZEL) run --config=mayberemote //proberk:pushk_proberk-base

run-local-instance:
ifndef K8S_CONFIG
	$(error K8S_CONFIG environment variable must be set)
endif
	$(BAZEL) run --config=mayberemote //proberk/go/proberk -- \
 		--local --run_every=1s \
		--config=$(K8S_CONFIG)/prober/generated/allprobersk.json \
		--expectations_dir=$(K8S_CONFIG)/prober/expectations
