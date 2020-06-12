.PHONY: all
all:
	npm ci

# For local development only. Runs any Puppeteer tests inside //puppeteer-tests,
# e.g. //puppeteer-tests/util_test.ts.
.PHONY: test
test:
	npx mocha -r ts-node/register ./**/*_test.ts
