build: core
	npx webpack --mode=development

serve:
	npx webpack-dev-server --watch

core:
	go install -v ./go/coverage-server

release_build: core
	npx webpack --mode=production

release: release_build
	./coverage_release "`git log -n1 --format=%s`"

push: release
	pushcli coverage-ingest skia-coverage

watch: core
	npx webpack --watch

.PHONY: build serve core release_build release push watch
