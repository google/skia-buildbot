default:
	go generate .
	go install .

.PHONY: debug
debug: npm-ci
	npx webpack-cli --mode=development

.PHONY: testjs
testjs: npm-ci
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: test-frontend-ci
test-frontend-ci: build-frontend-ci npm-ci
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=development

.PHONY: testpptr
testpptr: npm-ci
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha --require ts-node/register ./modules/**/*_puppeteer_test.ts

include ../make/npm.mk
