include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: build
build:
	$(BAZEL) build //proberk/go/...
	build_probersk_json --dest=allprobersk.json

.PHONY: validate
validate: build
	proberk --config=allprobersk.json --validate

.PHONY: release
release: build
	$(BAZEL) run //proberk:push_proberk_container

.PHONY: update-expectations
update-expectations:
	curl https://skia.googlesource.com?format=JSON > expectations/gob.json

.PHONY: push
push: pushk release
	pushk --use-temp-checkout proberk

run-local-instance: build
	proberk --local --run_every=1s --config=./allprobersk.json
