build:
	npx webpack --mode=development

serve: package-lock.json
	npx webpack-dev-server --watch-poll --mode=development

release_build:
	npx webpack --mode=production

watch:
	npx webpack --watch --mode=development

package-lock.json: package.json
	npm install

canvaskit:
	# Hack until canvaskit is on npm
	rm -rf modules/canvaskit
	mkdir -p modules/canvaskit
	wget -O modules/canvaskit/skia.js https://storage.googleapis.com/skia-cdn/canvaskit-wasm/0.0.3/bin/skia.js
	wget -O modules/canvaskit/skia.wasm https://storage.googleapis.com/skia-cdn/canvaskit-wasm/0.0.3/bin/skia.wasm

core:
	go install -v ./go/jsfiddle

release: release_build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/jsfiddle
	./build_release

push: release skia-public
	pushk jsfiddle

.PHONY: build serve release_build watch core release

include ../make/clusters.mk