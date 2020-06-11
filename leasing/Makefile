include ../kube/kube.mk

default: build

build:
	cd ../infra-sk && npm ci
	npm ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/leasing

release: build
	./build_docker_release

debug:
	npx webpack --mode=development

push: pushk release
	pushk --use-temp-checkout leasing

serve:
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

watch:
	npx webpack --mode=development --watch
