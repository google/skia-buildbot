include ../make/bazel.mk

.PHONY: build
build:
	-rm `pwd`/allprobersk.json
	$(BAZEL) build --config=mayberemote //proberk/go/...
	$(BAZEL) run --config=mayberemote //proberk/go/build_probersk_json:build_probersk_json5 -- \
 		--dest=`pwd`/allprobersk.json --srcdir $(realpath $(CURDIR)/..)

.PHONY: validate
validate: build
	$(BAZEL) run --config=mayberemote //proberk/go/proberk -- \
 		--config=`pwd`/allprobersk.json --validate

.PHONY: release
release: build
	$(BAZEL) run --config=mayberemote //proberk:push_proberk

.PHONY: update-expectations
update-expectations:
	curl https://skia.googlesource.com?format=JSON > expectations/gob.json

.PHONY: push
push: validate
	$(BAZEL) run --config=mayberemote //proberk:pushk_proberk

run-local-instance: validate
	$(BAZEL) run --config=mayberemote //proberk/go/proberk -- \
 		--local --run_every=1s --config=`pwd`/allprobersk.json --expectations_dir=`pwd`/expectations
