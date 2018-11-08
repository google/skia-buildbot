default: app
	npx webpack --mode=development

release: app
	docker pull gcr.io/skia-public/skia-release:prod
	npx webpack --mode=production
	./build_release

release_ci: app
	# Don't docker pull here, as we are in the CI pipeline and it already has
	# done the pull.
	npx webpack --mode=production
	./build_release

serve: app
	npx webpack-dev-server --mode=development --watch

watch: app
	npx webpack --mode=development --watch

app: package-lock.json
	go install ./go/skottie

package-lock.json: package.json
	npm install
	touch package-lock.json

push: release skia-public
	pushk skottie

wasm_libs:
	rm -rf build
	mkdir -p build/canvaskit
	# Extract them from the most recently built ones. These are built
	# using cloudbuild.yaml in the Skia repo.
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp -r /tmp/canvaskit /OUT/
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp /tmp/VERSION /OUT/
	echo "export const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

include ../make/clusters.mk
