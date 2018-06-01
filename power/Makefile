build: core
	npx webpack --mode=development

serve: package-lock.json
	npx webpack-dev-server --watch --mode=development

core: package-lock.json
	go install -v ./go/power-controller

release_build: core
	npx webpack --mode=production

release: release_build
	./power_release "`git log -n1 --format=%s`"

push: release
	pushcli power-controller skia-power-controller

watch: core
	npx webpack --watch --mode=development

package-lock.json: package.json
	npm install

.PHONY: build serve core release_build release push watch
