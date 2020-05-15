core:
	go install -v ./go/demoserver

serve:
	npx webpack-dev-server --watch-poll --mode-development

fetch_demos:
	go run ./go/load-demo-assets/main.go \
		--alsologtostderr \
		--demos_dir "demos/internal" \
		--out_dir "fetched_demos" \
		--repo_url "https://skia.googlesource.com/infra-internal"

wasm_libs:
	rm -rf build
	mkdir -p build/canvaskit build/pathkit
	# Extract them from the most recently built ones.
	# Run as current user so they have non-root ownership.
	docker run -u $$(id -u ${USER}):$$(id -g ${USER}) --rm \
		--volume `pwd`/build:/OUT gcr.io/skia-public/skia-wasm-release:prod \
		sh -c "cp -r /tmp/* /OUT/"
	echo "export const SKIA_VERSION = '`cat build/VERSION`';" > build/version.js

build_resources: wasm_libs
	npx webpack --mode=production

run: build_resources core
	demoserver \
	--alsologtostderr \
	--port=:8000 \
	--resources_dir="dist" \
	--demos_dir="fetched_demos"


release: build_resources fetch_demos
	CGO_ENABLED=0 GOOS=linux go install -a ./go/demoserver
	./build_release

local_image: build_resources fetch_demos
	CGO_ENABLED=0 GOOS=linux go install -a ./go/demoserver
	SKIP_UPLOAD=1 ./build_release

test_puppeteer:
	npx mocha ./**/*_puppeteer_test.js

test_js:
	npx webpack --mode=development
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

test: test_js test_puppeteer
