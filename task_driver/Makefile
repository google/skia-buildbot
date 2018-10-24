build: package-lock.json
	npx webpack --mode=development
	GOOS=linux go install -a ./go/task-driver-server

release: build package-lock.json
	NODE_ENV='production' npx webpack --mode=production
	GOOS=linux go install -a ./go/task-driver-server
	./build_release

serve: package-lock.json
	npx webpack-dev-server --mode=development --watch-poll --host=$(shell hostname)

watch: package-lock.json
	npx webpack --watch-poll --mode=development --watch-poll

package-lock.json: package.json
	npm install
