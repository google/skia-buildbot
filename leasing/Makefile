default: build

build:
	cd ../infra-sk && npm ci
	npm ci
	npx webpack --mode=production
	go install ./go/leasing

release: build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/leasing
	./build_docker_release

debug:
	npx webpack --mode=development

push: release
	pushk leasing

serve:
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

watch:
	npx webpack --mode=development --watch
