KGO := CGO_ENABLED=0 GOOS=linux go build

node_modules: package.json
	npm ci

.PHONY: test
test: go-test js-test
	true

.PHONY: go-test
go-test:
	go test ./go/...

.PHONY: js-test
js-test:
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: js-test-debug
js-test-debug:
	npx karma start --no-single-run

.PHONY: puppeteer-test
puppeteer-test:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

.PHONY: puppeteer-test-debug
puppeteer-test-debug:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts --inspect-brk

.PHONY: serve
serve:
	npx webpack-dev-server --watch-poll --mode=development

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci:
	npm ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run
