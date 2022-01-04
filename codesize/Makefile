include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build ...

.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/codesize/go/codesizeserver/codesizeserver_/codesizeserver --local --resources_dir ../_bazel_bin/codesize/pages/development
