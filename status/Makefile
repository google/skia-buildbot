include ../kube/kube.mk

.PHONY: all
all: status

.PHONY: serve
serve:
	npm ci
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/... -v

testjs:
	npm ci
	npx webpack --mode=development
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

test-frontend-ci: testjs

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: puppeteer-test
puppeteer-test:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts
	echo "results are found in ../puppeteer-tests/output"

.PHONY: puppeteer-test-debug
puppeteer-test-debug:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts --inspect-brk
	echo "results are found in ../puppeteer-tests/output"

status: build-frontend-ci
	CGO_ENABLED=0 GOOS=linux go install -a ./go/status

release: status
	./build_release

run_docker: release
	docker run status

push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout status


.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=production
