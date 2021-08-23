core:
	go install -v ./go/demoserver

serve: npm-ci
	npx webpack-dev-server --watch-poll --mode=development

build_resources: npm-ci
	npx webpack --mode=production

run: build_resources core
	demoserver \
	--local \
	--port=:8000 \
	--resources_dir="dist"

release: build_resources
	CGO_ENABLED=0 GOOS=linux go install -a ./go/demoserver
	./build_release

local_image: build_resources
	CGO_ENABLED=0 GOOS=linux go install -a ./go/demoserver
	SKIP_UPLOAD=1 ./build_release

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

test_puppeteer: npm-ci
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

test_js: npm-ci
	npx webpack --mode=development
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

test: test_js test_puppeteer

push: release
	pushk demoserver

.PHONY: build-frontend-ci
build-frontend-ci: wasm_libs npm-ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

include ../make/npm.mk
