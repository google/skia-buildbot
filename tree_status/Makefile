default: build
	cd .. && npm ci
	npx webpack --mode=development

./modules/json/index.ts:
	go generate ./go/types/ts

build: ./modules/json/index.ts
	go install ./go/tree_status

release: build
	cd .. && npm ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/tree_status
	./build_tree_status_release

debug:
	npx webpack --mode=development

push: release
	pushk tree_status

serve:
	cd .. && npm ci
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

watch:
	cd .. && npm ci
	npx webpack --mode=development --watch

.PHONY: build-frontend-ci
build-frontend-ci:
	npx webpack --mode=production
