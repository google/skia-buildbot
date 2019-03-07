default: app
	npx webpack --mode=development

release: wasm_libs_tot
	npx webpack --mode=production
	./build_release

serve: app
	npx webpack-dev-server --mode=development --watch --watch-poll 100

watch: app
	npx webpack --mode=development --watch

package-lock.json: package.json
	npm install
	touch package-lock.json

get_latest_skia:
	docker pull gcr.io/skia-public/skia-wasm-release:prod

wasm_libs_tot: get_latest_skia
	rm -rf build
	mkdir -p build/canvaskit
	# Extract them from the most recently built ones. These are built
	# using cloudbuild.yaml in the Skia repo.
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp -r /tmp/canvaskit /OUT/
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp /tmp/VERSION /OUT/
	echo "export const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

wasm_libs_fixed: package-lock.json
	rm -rf build
	mkdir -p build/canvaskit
	cp node_modules/canvaskit-wasm/bin/* build/canvaskit
	echo "export const SKIA_VERSION = '0.4.2';" > build/version.js

.PHONY: serve watch app release wasm_libs_fixed wasm_libs_tot get_latest_skia push

