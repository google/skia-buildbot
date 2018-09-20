build:
	npx webpack --mode=development

serve: package-lock.json
	npx webpack-dev-server --watch-poll --mode=development

release_build:
	npx webpack --mode=production

watch:
	npx webpack --watch --mode=development

package-lock.json: package.json
	npm install

core:
	go install -v ./go/jsfiddle

release: release_build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/jsfiddle
	./build_release

push: release
	pushk jsfiddle

.PHONY: build serve release_build watch core release

include ../make/clusters.mk