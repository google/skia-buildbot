.PHONY: test-frontend-ci
test-frontend-ci: npm-ci
	npx mocha -r ts-node/register ./**/*_test.ts

include ../make/npm.mk
