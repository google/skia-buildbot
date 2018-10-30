web: package-lock.json
	npx webpack --mode=development

web-prod: package-lock.json
	NODE_ENV='production' npx webpack --mode=production

build: web
	GOOS=linux go install -a ./go/task-driver-server

release: build web-prod
	GOOS=linux go install -a ./go/task-driver-server
	./build_release

serve: package-lock.json
	npx webpack-dev-server --mode=development --watch-poll --host=$(shell hostname)

watch: package-lock.json
	npx webpack --watch-poll --mode=development --watch-poll

package-lock.json: package.json
	npm install
