include ../make/bazel.mk

.PHONY: build
build: build-placeholders
	$(BAZEL) build --config=mayberemote //debugger-app/...

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //debugger-app:push_debugger_container-base
	$(MAKE) build-placeholders

.PHONY: get_latest_skia
get_latest_skia:
	docker pull gcr.io/skia-public/skia-wasm-release:prod
	rm -rf build
	# Make docker happy by having these folders already exist.
	mkdir -p build/debugger
	# Extract them from the most recently built docker container. This is built
	# with @skia//docker/skia-wasm-release/Dockerfile
	docker run -u $$(id -u ${USER}):$$(id -g ${USER}) --rm \
	    --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod \
	    sh -c "cp -r /tmp/canvaskit /tmp/VERSION /OUT/"
	echo "const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

# Default port is 8000, but setting this environment variable can override that
# https://stackoverflow.com/a/53370979
DEBUGGER_LOCAL_PORT ?= :8000
.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/debugger-app/go/debugger-app/debugger-app_/debugger-app \
        --resources_dir ../_bazel_bin/debugger-app/pages/development \
        --port ${DEBUGGER_LOCAL_PORT}

.PHONY: run-with-custom
run-with-custom:
	echo "Using the debugger files in //debugger-app/wasm_libs/local_build"
	$(BAZEL) build --config=mayberemote //debugger-app/... --with_local_debugger
	../_bazel_bin/debugger-app/go/debugger-app/debugger-app_/debugger-app \
        --resources_dir ../_bazel_bin/debugger-app/pages/development \
        --port ${DEBUGGER_LOCAL_PORT}

build-placeholders:
	rm -rf build
	mkdir -p build/canvaskit
	touch build/canvaskit/canvaskit.js
	touch build/canvaskit/canvaskit.wasm
	touch build/version.js