include ../make/bazel.mk

.PHONY: build
build: build-placeholders
	$(BAZEL) build --config=mayberemote //skottie/...

# See README.md for further information on this target.
.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //skottie:push_skottie_container-base

.PHONY: gazelle
gazelle:
	$(BAZEL) run //:gazelle -- update ./

.PHONY: test
test:
	$(BAZEL) test --config=mayberemote --test_output=errors //skottie/...

.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/skottie/go/skottie/skottie_/skottie \
		--config ./local_config.json5 \
		--internal_site_domain skottie-internal.corp.goog

.PHONY: run-with-custom
run-with-custom:
	echo "Using the wasm files in //skottie/wasm_libs/local_build"
	$(BAZEL) build --config=mayberemote //skottie/... \
		--//skottie/wasm_libs:use_canvaskit_from_container=False
	../_bazel_bin/skottie/go/skottie/skottie_/skottie \
		--config ./local_config.json5

build-placeholders:
	rm -rf build
	mkdir -p build/canvaskit
	touch build/canvaskit/canvaskit.js
	touch build/canvaskit/canvaskit.wasm
	touch build/version.js


# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/skottie/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles) Makefile
	go generate ./go/ts

.PHONY: watch-modules
watch-modules:
	while true; do inotifywait -r --event create,delete,move,modify modules/* && $(BAZEL) build --config=mayberemote //skottie/pages:index; done;
