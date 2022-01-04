include ../make/bazel.mk

.PHONY: build
build: build-placeholders
	$(BAZEL) build //debugger-app/...

.PHONY: release
release: get_latest_skia
	$(BAZEL) run //debugger-app:push_debugger_container
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
	    sh -c "cp -r /tmp/debugger /tmp/VERSION /OUT/"
	echo "const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

.PHONY: push
push: release
	pushk debugger

bazel_release_ci:
	# COPY_FROM_DIR is expected to be set by the auto deploy mechanism, e.g.
	# https://github.com/google/skia/blob/44c81d1492738cb1c6ab3d68a925bd7e64201bca/infra/bots/task_drivers/push_bazel_apps_from_wasm_image/push_bazel_apps_from_wasm_image.go#L137
	cp -r $(COPY_FROM_DIR)/* build/
	echo "const SKIA_VERSION = '`cat ./build/VERSION`';" > ./build/version.js
	bazelisk run //debugger-app:push_debugger_container --workspace_status_command=bazel/override_container_label.sh
	$(MAKE) build-placeholders

.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/debugger-app/go/debugger-app/debugger-app_/debugger-app \
        --resources_dir ../_bazel_bin/debugger-app/pages/development

build-placeholders:
	rm -rf build
	mkdir -p build/debugger
	touch build/debugger/debugger.js
	touch build/debugger/debugger.wasm
	touch build/version.js