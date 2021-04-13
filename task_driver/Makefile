include ../kube/kube.mk

.PHONY: build
build:
	npx webpack --mode=development
	GOOS=linux go install -a ./go/task-driver-server

.PHONY: release
release:
	NODE_ENV='production' npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-driver-server
	./build_release

.PHONY: push
push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout task-driver-server

.PHONY: serve
serve:
	npx webpack-dev-server --mode=development --watch-poll --host=$(shell hostname)

.PHONY: watch
watch:
	npx webpack --watch-poll --mode=development --watch-poll

.PHONY: build-frontend-ci
build-frontend-ci:
	npx webpack --mode=production
