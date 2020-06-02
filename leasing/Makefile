default: build package-lock.json
	npx webpack --mode=development

build:
	go install ./go/leasing

release: build package-lock.json
	cd ../infra-sk && npm ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/leasing
	./build_docker_release

debug:
	npx webpack --mode=development

push: release
	pushk leasing

serve: package-lock.json
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

watch: package-lock.json
	npx webpack --mode=development --watch

package-lock.json: package.json
	npm install
