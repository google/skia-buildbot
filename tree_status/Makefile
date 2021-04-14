default: build npm-ci
	npx webpack --mode=development

./modules/json/index.ts:
	go generate ./go/types/ts

build: ./modules/json/index.ts
	go install ./go/tree_status

release: build npm-ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/tree_status
	./build_tree_status_release

debug: npm-ci
	npx webpack --mode=development

push: release
	pushk tree_status

serve: npm-ci
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

watch: npm-ci
	npx webpack --mode=development --watch

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

include ../make/npm.mk
