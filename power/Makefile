build: core
	npx webpack

serve:
	npx webpack-dev-server --watch

core:
	yarn
	go install -v ./go/power-controller

release_build: core
	NODE_ENV=production npx webpack

release: release_build
	./power_release "`git log -n1 --format=%s`"

push: release
	pushcli power-controller skia-power-controller

watch: core
	npx webpack --watch

.PHONY: build serve core release_build release push watch