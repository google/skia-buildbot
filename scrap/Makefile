build: web
	go install -v ./go/...

web: package-lock.json
	npx webpack --mode=development

test:
	go test -cover ./go/...
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

testjs:
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: testpptr
testpptr:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

debugjs:
	npx karma start --no-single-run

testci:
	rm -rf node_modules
	npm install
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

release: build package-lock.json
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/scrapexchange
	./images/scrapexchange/release

watch: package-lock.json
	npx webpack --mode=development --watch

push: release
	pushk scrapexchange

package-lock.json: package.json
	npm install

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci:
	npm ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

start-local-server:
	 scrapexchange --local --logtostderr --bucket=skia-public-scrap-exchange