default: build web

web: ./modules/json/index.ts npm-ci
	npx webpack --mode=development

./modules/json/index.ts: ./go/types/types.go
	go run ./go/types/ts

build: web
	go install -v ./go/bugs-central

release: build npm-ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/bugs-central
	./images/bugs-central-server/build_release

debug: npm-ci
	npx webpack --mode=development

push: release
	pushk --use-temp-checkout bugs-central

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
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha --require ts-node/register ./**/*_puppeteer_test.ts

include ../make/npm.mk
