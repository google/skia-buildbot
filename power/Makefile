build: core npm-ci
	npx webpack --mode=development

serve: npm-ci
	npx webpack-dev-server --watch --mode=development

core: npm-ci
	go install -v ./go/power-controller

release_build: npm-ci
	npx webpack --mode=production

release: release_build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/power-controller
	./power_release "`git log -n1 --format=%s`"

push: release
	pushk power-controller

watch: core npm-ci
	npx webpack --watch --mode=development

.PHONY: build serve core release_build release push watch

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

include ../make/npm.mk
