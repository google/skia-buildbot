all: default

alertserver: js
	go install -v ./go/alertserver

datahopper:
	go install -v ./go/datahopper

grains:
	go install -v ./go/grains

default: alertserver datahopper grains

testgo:
	go test ./go/...

node_modules: package.json
	npm install

js: node_modules
	./node_modules/.bin/grunt --no-color

res/js/perf.js: node_modules Gruntfile.js bower.json res/js/common.js
	./node_modules/.bin/grunt --no-color

debug-js:
	./node_modules/.bin/grunt --no-color debug-js

lint:
	grunt --no-color jshint:main
