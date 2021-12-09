include ../make/bazel.mk

build: npm-ci
	go install ./go/...
	npx webpack --mode=development

serve: npm-ci
	npx webpack-dev-server --watch-poll --mode=development

release_build: wasm_libs npm-ci
	npx webpack --mode=production

watch: npm-ci
	npx webpack --watch --mode=development

wasm_libs:
	docker pull gcr.io/skia-public/skia-wasm-release:prod
	rm -rf build/canvaskit build/pathkit build/debugger
	mkdir -p build/canvaskit build/pathkit
	# Extract them from the most recently built ones. These are built
	# using cloudbuild.yaml in the Skia repo.
	docker run -u $$(id -u ${USER}):$$(id -g ${USER}) --rm \
	    --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod \
	    sh -c "cp -r /tmp/* /OUT/"
	echo "export const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

.PHONY: testjs
testjs: npm-ci
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

core:
	go install -v ./go/jsfiddle

release: release_build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/jsfiddle
	./build_release

release_ci: npm-ci
	# Assumes the environment mounts the wasm outputs to /OUT
	rm -rf build
	mkdir -p build/canvaskit build/pathkit
	cp -r /OUT/* build
	echo "export const SKIA_VERSION = '`cat /OUT/VERSION`';" > ./build/version.js
	# Now that we have the canvaskit.wasm etc, we can bundle it in with webpack.
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/jsfiddle
	./build_release

push: release
	pushk jsfiddle

bazel_release: wasm_libs
	$(BAZEL) run //jsfiddle:push_jsfiddle_container

bazel_release_ci:
	# COPY_FROM_DIR is expected to be set by the auto deploy mechanism, e.g.
	# https://github.com/google/skia/blob/44c81d1492738cb1c6ab3d68a925bd7e64201bca/infra/bots/task_drivers/push_bazel_apps_from_wasm_image/push_bazel_apps_from_wasm_image.go#L137
	cp -r $(COPY_FROM_DIR)/* build/
	echo "export const SKIA_VERSION = '`cat ./build/VERSION`';" > ./build/version.js
	bazelisk run //jsfiddle:push_jsfiddle_container --workspace_status_command=bazel/override_container_label.sh

run-local-instance: wasm_libs
	$(BAZEL) build ...
	../_bazel_bin/jsfiddle/go/jsfiddle/jsfiddle_/jsfiddle --local --resources_dir ../_bazel_bin/jsfiddle/pages/development
	# put the placeholders back
	git checkout origin/main -- build/

.PHONY: build serve release_build watch core release wasm_libs

.PHONY: build-frontend-ci
build-frontend-ci: wasm_libs npm-ci
	npx webpack --mode=production

include ../make/npm.mk
