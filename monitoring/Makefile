include ../go/skiaversion/skiaversion.mk

all: default

alertserver: js skiaversion
	go install -v ./go/alertserver

default: alertserver

testgo: skiaversion
	go test ./go/... -v

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
