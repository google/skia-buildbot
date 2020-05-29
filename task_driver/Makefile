include ../kube/kube.mk

.PHONY: build
build: package-lock.json
	npx webpack --mode=development
	GOOS=linux go install -a ./go/task-driver-server

.PHONY: release
release: package-lock.json
	NODE_ENV='production' npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-driver-server
	./build_release

.PHONY: push
push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout task-driver-server

.PHONY: serve
serve: package-lock.json
	npx webpack-dev-server --mode=development --watch-poll --host=$(shell hostname)

.PHONY: watch
watch: package-lock.json
	npx webpack --watch-poll --mode=development --watch-poll

package-lock.json: package.json
	npm install
