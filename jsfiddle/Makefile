include ../make/bazel.mk

build: wasm_libs
	$(BAZEL) build //jsfiddle/...

push: release
	pushk jsfiddle

release: wasm_libs
	$(BAZEL) run //jsfiddle:push_jsfiddle_container

bazel_release_ci:
	# COPY_FROM_DIR is expected to be set by the auto deploy mechanism, e.g.
	# https://github.com/google/skia/blob/44c81d1492738cb1c6ab3d68a925bd7e64201bca/infra/bots/task_drivers/push_bazel_apps_from_wasm_image/push_bazel_apps_from_wasm_image.go#L137
	cp -r $(COPY_FROM_DIR)/* build/
	echo "export const SKIA_VERSION = '`cat ./build/VERSION`';" > ./build/version.js
	bazelisk run //jsfiddle:push_jsfiddle_container --workspace_status_command=bazel/override_container_label.sh

run-local-instance: build
	../_bazel_bin/jsfiddle/go/jsfiddle/jsfiddle_/jsfiddle --local --resources_dir ../_bazel_bin/jsfiddle/pages/development
	# put the placeholders back
	git checkout origin/main -- build/

.PHONY: build release wasm_libs

