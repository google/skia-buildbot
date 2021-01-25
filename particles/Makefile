default: app
	npx webpack --mode=development

release: wasm_libs_tot
	CGO_ENABLED=0 GOOS=linux go install -a ./go/particles
	npx webpack --mode=production
	./build_release

release_ci: app
	cd ../infra-sk && npm ci
	npm ci
	# Assumes the cloud builder environment mounts the wasm outputs to /OUT
	rm -rf build
	mkdir -p build/canvaskit
	cp -r /OUT/* build
	echo "export const SKIA_VERSION = '`cat /OUT/VERSION`';" > ./build/version.ts
	# Now that we have the canvaskit.wasm etc, we can bundle it in with webpack.
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/particles
	./build_release

serve:
	npx webpack-dev-server --mode=development --watch --watch-poll 100

watch:
	npx webpack --mode=development --watch

app:
	go install ./go/particles

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: testpptr
testpptr:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

get_latest_skia:
	docker pull gcr.io/skia-public/skia-wasm-release:prod

wasm_libs_tot: get_latest_skia
	rm -rf build
	mkdir -p build/canvaskit
	# Extract them from the most recently built ones. These are built
	# using cloudbuild.yaml in the Skia repo.
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp -r /tmp/canvaskit /OUT/
	docker run --rm --volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod cp /tmp/VERSION /OUT/
	echo "export const SKIA_VERSION = '`cat build/VERSION`';" > build/version.ts

wasm_libs_fixed: package-lock.json
	rm -rf build
	mkdir -p build/canvaskit
	# TODO(jcgregorio) Change /static/ to /dist/ once the new particles.skia.org lands.
	curl https://particles.skia.org/static/canvaskit.js --output build/canvaskit/canvaskit.js
	curl https://particles.skia.org/static/canvaskit.wasm --output build/canvaskit/canvaskit.wasm
	# TODO(jcgregorio) Remove once particles.skia.org migration to TS lands.
	cp tmp/canvaskit.d.ts build/canvaskit/canvaskit.d.ts
	echo "Latest" >> build/VERSION
	echo "export const SKIA_VERSION = '`cat build/VERSION`';" > build/version.ts

push: release skia-public
	pushk particles

.PHONY: serve watch app release release_ci wasm_libs_fixed wasm_libs_tot get_latest_skia push

.PHONY: build-frontend-ci
build-frontend-ci: wasm_libs_tot
	npm ci
	npx webpack --mode=production

# You can either run a local copy of scrapexchange, or port-forward the production instance:
#
#   kubectl port-forward service/scrapexchange 9000
run-local-instance:
	particles --local --logtostderr --scrapexchange=http://localhost:9000