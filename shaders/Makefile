include ../make/bazel.mk

PORT?=:8001
PROM_PORT?=:20001

.PHONY: build
build:
	$(BAZEL) build --config=mayberemote //shaders/...

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //shaders:push_shaders_container-base

.PHONY: wasm_libs
wasm_libs:
	docker pull gcr.io/skia-public/skia-wasm-release:prod
	rm -rf build/canvaskit
	mkdir -p build/canvaskit
	# Extract them from the most recently built docker container. This is built
	# with @skia//docker/skia-wasm-release/Dockerfile
	docker run -u $$(id -u ${USER}):$$(id -g ${USER}) --rm \
	    --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod \
	    sh -c "cp -r /tmp/canvaskit /tmp/VERSION /OUT/"
	echo "const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

.PHONY: release-base
release-base:
	$(BAZEL) run --config=mayberemote //shaders:push_shaders_container-base

# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/shaders/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles) Makefile
	go generate ./go/ts

# You can either run a local copy of scrapexchange, or port-forward the production instance:
#
#   kubectl port-forward service/scrapexchange 9000
.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/shaders/go/shaders/shaders_/shaders --local \
		--resources_dir ../_bazel_bin/shaders/pages/development \
		--scrapexchange=http://localhost:9000 \
		--port=$(PORT) --prom_port=$(PROM_PORT)

.PHONY: run-with-custom
run-with-custom:
	echo "Using the wasm files in //shaders/wasm_libs/local_build and an in-memory scrapexchange"
	$(BAZEL) build --config=mayberemote //shaders/... --shaders_local_canvaskit
	../_bazel_bin/shaders/go/shaders/shaders_/shaders --local \
		--resources_dir ../_bazel_bin/shaders/pages/development \
		--fake_scrapexchange \
		--port=:8001 \
		--prom_port=:20001

.PHONY: build-placeholders
build-placeholders:
	rm -rf build
	mkdir -p build/canvaskit
	touch build/canvaskit/canvaskit.js
	touch build/canvaskit/canvaskit.wasm
	touch build/version.js

testgo:
	go test ./go/...