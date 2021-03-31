# The "all" target is invoked from the "all" target in the Makefile in the repo's root directory,
# which is in turn invoked by the Infra-PerCommit-Build task to build various Skia Infrastructure
# apps. Since some of those apps depend on infra-sk, we first need to install the npm dependencies
# and make sure infra-sk actually builds.
all: build-frontend-ci

default: package-lock.json
	npx webpack --mode=development

watch: package-lock.json
	npx webpack --mode=development --watch

release: package-lock.json
	npx webpack --mode=production

serve: package-lock.json
	npx webpack-dev-server --mode=development --content-base ./dist --watch-poll

test-js:
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

debug-test-js:
	npx karma start --no-single-run

docs: package-lock.json
	npx jsdoc -c jsdoc.config.js

package-lock.json: package.json
	npm install

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	# Webpack fails with --mode=production because there are no production pages under //infra-sk,
	# and therefore nothing to build.
	npx webpack --mode=development

.PHONY: test-frontend-ci
test-frontend-ci:
	npm ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: puppeteer_tests
puppeteer_tests:
	npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

test: test-js puppeteer_tests
