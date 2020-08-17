build: core
	npx webpack --mode=development

serve: package-lock.json
	npx webpack-dev-server --watch --mode=development

core: package-lock.json
	go install -v ./go/power-controller

release_build: package-lock.json
	npx webpack --mode=production

release: release_build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/power-controller
	./power_release "`git log -n1 --format=%s`"

push: release
	pushk power-controller

watch: core
	npx webpack --watch --mode=development

package-lock.json: package.json
	npm install

.PHONY: build serve core release_build release push watch

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=production
