build:
	npx webpack --mode=development

serve: package-lock.json
	npx webpack-dev-server --watch-poll --mode=development

release_build: wasm_libs
	npx webpack --mode=production

watch:
	npx webpack --watch --mode=development

package-lock.json: package.json
	npm install

wasm_libs:
	rm -rf build
	mkdir build
	# Extract them from the most recently built ones. These are built
	# using cloudbuild.yaml in the Skia repo.
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp /tmp/skia/skia/out/canvaskit_wasm/canvaskit.js /OUT/canvaskit.js
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp /tmp/skia/skia/out/canvaskit_wasm/canvaskit.wasm /OUT/canvaskit.wasm
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp /tmp/skia/skia/out/pathkit/pathkit.js /OUT/pathkit.js
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp /tmp/skia/skia/out/pathkit/pathkit.wasm /OUT/pathkit.wasm
	docker run --rm --volume `pwd`/build:/OUT --workdir /tmp/skia/skia gcr.io/skia-public/skia-wasm-release:prod sh -c "git rev-parse HEAD > /OUT/VERSION"
	echo "export const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

core:
	go install -v ./go/jsfiddle

release: release_build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/jsfiddle
	./build_release

push: release skia-public
	pushk jsfiddle

.PHONY: build serve release_build watch core release wasm_libs

include ../make/clusters.mk