default: build package-lock.json
	npx webpack --mode=development

build:
	go install ./go/tree_status

release: build package-lock.json
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/tree_status
	./build_tree_status_release

debug:
	npx webpack --mode=development

push: release
	pushk tree_status

serve: package-lock.json
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

watch: package-lock.json
	npx webpack --mode=development --watch

package-lock.json: package.json
	npm install

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=production
