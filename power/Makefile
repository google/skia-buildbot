build: core
	npx webpack --mode=development

serve:
	npx webpack-dev-server --watch --mode=development

core:
	go install -v ./go/power-controller

release_build:
	cd .. && npm ci
	npx webpack --mode=production

release: release_build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/power-controller
	./power_release "`git log -n1 --format=%s`"

push: release
	pushk power-controller

watch: core
	npx webpack --watch --mode=development

.PHONY: build serve core release_build release push watch

.PHONY: build-frontend-ci
build-frontend-ci:
	npx webpack --mode=production
