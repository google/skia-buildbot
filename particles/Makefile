include ../make/bazel.mk

.PHONY: build
build: wasm_libs
	$(BAZEL) build //particles/...

.PHONY: wasm_libs
wasm_libs:
	docker pull gcr.io/skia-public/skia-wasm-release:prod
	rm -rf build/canvaskit
	mkdir -p build/canvaskit
	# Extract them from the most recently built docker container. This is built
	# with @skia//docker/skia-wasm-release/Dockerfile
	docker run -u $$(id -u ${USER}):$$(id -g ${USER}) --rm \
	    --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod \
	    sh -c "cp -r /tmp/canvaskit /OUT/"
	echo "export const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

.PHONY: release
release: wasm_libs
	$(BAZEL) run //particles:push_particles_container

.PHONY: push
push: release
	pushk particles

bazel_release_ci:
	# COPY_FROM_DIR is expected to be set by the auto deploy mechanism, e.g.
	# https://github.com/google/skia/blob/44c81d1492738cb1c6ab3d68a925bd7e64201bca/infra/bots/task_drivers/push_bazel_apps_from_wasm_image/push_bazel_apps_from_wasm_image.go#L137
	cp -r $(COPY_FROM_DIR)/* build/
	echo "export const SKIA_VERSION = '`cat ./build/VERSION`';" > ./build/version.js
	bazelisk run //particles:push_particles_container --workspace_status_command=bazel/override_container_label.sh

# You can either run a local copy of scrapexchange, or port-forward the production instance:
#
#   kubectl port-forward service/scrapexchange 9000
.PHONY: run-local-instance
run-local-instance: build
	../_bazel_bin/particles/go/particles/particles_/particles --local \
		--resources_dir ../_bazel_bin/particles/pages/development \
		--scrapexchange=http://localhost:9000
	# put the placeholders back
	git checkout origin/main -- build/
