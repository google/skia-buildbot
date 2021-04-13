include ../kube/kube.mk

default: build

./modules/json/index.ts:
	go run ./go/types/ts -o modules/json/index.ts

build: ./modules/json/index.ts npm-ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/leasing

release: build
	./build_docker_release

debug: npm-ci
	npx webpack --mode=development

push: pushk release
	pushk --use-temp-checkout leasing

serve: npm-ci
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

watch: npm-ci
	npx webpack --mode=development --watch

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

include ../make/npm.mk
