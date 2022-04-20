include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build ...

.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/codesize/go/codesizeserver/codesizeserver_/codesizeserver --local --resources_dir ../_bazel_bin/codesize/pages/development

.PHONY: entr-local-instance
watch-local-instance:
	find . -type f | entr -r -s "bazel build //codesize/... && ../_bazel_bin/codesize/go/codesizeserver/codesizeserver_/codesizeserver --local --resources_dir ../_bazel_bin/codesize/pages/development"

release:
	$(BAZEL) run //codesize:push_codesizeserver

push: release
	$(BAZEL) run //kube/go/pushk -- codesizeserver