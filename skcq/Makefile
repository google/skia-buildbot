default: build-be web

web: ./modules/json/index.ts npm-ci
	npx webpack --mode=development

./modules/json/index.ts: ./go/types/types.go
	go generate ./go/types/ts

build-be:
	go install -v ./go/skcq-be

release-be: build-be
	CGO_ENABLED=0 GOOS=linux go install -a ./go/skcq-be
	./images/skcq-be/build_release

push-be: release-be
	pushk --use-temp-checkout skcq-be

debug: npm-ci
	npx webpack --mode=development

serve: npm-ci
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

watch: npm-ci
	npx webpack --mode=development --watch

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

.PHONY: test_js
test_js: npm-ci
	npx webpack --mode=development
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: puppeteer-tests
puppeteer-tests: npm-ci
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

include ../make/npm.mk
