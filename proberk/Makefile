include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build //proberk/go/...
	$(BAZEL) run //proberk/go/build_probersk_json:build_probersk_json5 -- \
 		--dest=allprobersk.json --srcdir `pwd`/..

.PHONY: validate
validate: build
	$(BAZEL) run //proberk/go/proberk -- \
 		--config=`pwd`/allprobersk.json --validate

.PHONY: release
release: build
	$(BAZEL) run //proberk:push_proberk_container

.PHONY: update-expectations
update-expectations:
	curl https://skia.googlesource.com?format=JSON > expectations/gob.json

.PHONY: push
push: pushk release
	$(BAZEL) run //kube/pushk -- --use-temp-checkout proberk

run-local-instance: build
	$(BAZEL) run //proberk/go/proberk -- \
 		--local --run_every=1s --config=`pwd`/allprobersk.json --expectations_dir=`pwd`/expectations
