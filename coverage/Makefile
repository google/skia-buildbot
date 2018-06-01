build: core
	npx webpack --mode=development

serve: package-lock.json
	npx webpack-dev-server --watch --mode=development

core: package-lock.json
	go install -v ./go/coverage-server

release_build: core
	npx webpack --mode=production

release: release_build package-lock.json
	./coverage_release "`git log -n1 --format=%s`"

push: release
	pushcli coverage-ingest skia-coverage

watch: core
	npx webpack --watch

package-lock.json: package.json
	npm install

.PHONY: build serve core release_build release push watch
