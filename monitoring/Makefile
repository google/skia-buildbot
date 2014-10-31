all: default

alertserver:
	go install -v ./go/alertserver

grains:
	go install -v ./go/grains

default: alertserver grains

testgo:
	go test ./go/...

node_modules: package.json
	npm install

js:
	./node_modules/.bin/grunt --no-color

res/js/perf.js: node_modules Gruntfile.js bower.json res/js/common.js
	./node_modules/.bin/grunt --no-color

debug-js:
	./node_modules/.bin/grunt --no-color debug-js

lint:
	grunt --no-color jshint:main
