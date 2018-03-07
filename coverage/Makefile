build: core
	npx webpack

serve:
	yarn
	npx webpack-dev-server --watch

core:
	yarn
	go install -v ./go/coverage-server

release_build: core
	NODE_ENV=production npx webpack

release: release_build
	./coverage_release "`git log -n1 --format=%s`"

push: release
	pushcli coverage-ingest skia-coverage

watch: core
	npx webpack --watch

.PHONY: build serve core release_build release push watch