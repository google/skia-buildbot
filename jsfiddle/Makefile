include ../make/bazel.mk

PORT?=:8000
PROM_PORT?=:20000

.PHONY: build
build: build-placeholders
	$(BAZEL) build --config=mayberemote //jsfiddle/...

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //jsfiddle:push_jsfiddle_container-base

.PHONY: wasm_libs
wasm_libs:
	docker pull gcr.io/skia-public/skia-wasm-release:prod
	rm -rf build/canvaskit build/debugger
	mkdir -p build/canvaskit
	# Extract them from the most recently built docker container. This is built
	# with @skia//docker/skia-wasm-release/Dockerfile
	docker run -u $$(id -u ${USER}):$$(id -g ${USER}) --rm \
	    --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod \
	    sh -c "cp -r /tmp/* /OUT/"
	echo "const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/jsfiddle/go/jsfiddle/jsfiddle_/jsfiddle --local \
	--resources_dir ../_bazel_bin/jsfiddle/pages/development \
	--scrapexchange http://localhost:9000 --port=$(PORT) \
	--prom_port=$(PROM_PORT)

.PHONY: run-with-custom
run-with-custom:
	echo "Using the wasm files in //jsfiddle/wasm_libs/local_build"
	$(BAZEL) build --config=mayberemote //jsfiddle/... --with_local_wasm
	../_bazel_bin/jsfiddle/go/jsfiddle/jsfiddle_/jsfiddle \
        --local --resources_dir ../_bazel_bin/jsfiddle/pages/development

.PHONY: build-placeholders
build-placeholders:
	rm -rf build
	mkdir -p build/canvaskit
	touch build/canvaskit/canvaskit.js
	touch build/canvaskit/canvaskit.wasm
	touch build/version.js

.PHONY: generate
generate:
		go generate ./go/...

.PHONY: testgo
testgo:
	go test -v ./...