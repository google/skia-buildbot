include ../make/bazel.mk

.PHONY: build
build: build-placeholders
	$(BAZEL) build --config=mayberemote //skottie/...

.PHONY: release
release: get_latest_skia
	$(BAZEL) run --config=mayberemote //skottie:push_skottie_container
	$(MAKE) build-placeholders

# See README.md for further information on this target.
.PHONY: release-base
release-base:
	$(BAZEL) run --config=mayberemote //skottie:push_skottie_container-base

.PHONY: gazelle
gazelle:
	$(BAZEL) run //:gazelle -- update ./

.PHONY: get_latest_skia
get_latest_skia:
	docker pull gcr.io/skia-public/skia-wasm-release:prod
	rm -rf build
	# Make docker happy by having these folders already exist.
	mkdir -p build/canvaskit
	# Extract them from the most recently built docker container. This is built
	# with @skia//docker/skia-wasm-release/Dockerfile
	docker run -u $$(id -u ${USER}):$$(id -g ${USER}) --rm \
	    --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod \
	    sh -c "cp -r /tmp/canvaskit /tmp/VERSION /OUT/"
	echo "const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

.PHONY: push
push: release
	pushk skottie

.PHONY: test
test:
	$(BAZEL) test --config=mayberemote --test_output=errors //skottie/...

bazel_release_ci:
	# COPY_FROM_DIR is expected to be set by the auto deploy mechanism, e.g.
	# https://github.com/google/skia/blob/44c81d1492738cb1c6ab3d68a925bd7e64201bca/infra/bots/task_drivers/push_bazel_apps_from_wasm_image/push_bazel_apps_from_wasm_image.go#L137
	cp -r $(COPY_FROM_DIR)/* build/
	echo "const SKIA_VERSION = '`cat ./build/VERSION`';" > ./build/version.js
	bazelisk run //skottie:push_skottie_container --workspace_status_command=bazel/override_container_label.sh

	$(MAKE) build-placeholders

.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/skottie/go/skottie/skottie_/skottie \
		--config ./local_config.json5 \
		--internal_site_domain skottie-internal.corp.goog

.PHONY: run-with-custom
run-with-custom:
	echo "Using the wasm files in //skottie/wasm_libs/local_build"
	$(BAZEL) build --config=mayberemote //skottie/... --skottie_local_canvaskit
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
