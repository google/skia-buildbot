alertserver:
	go install -v ./go/alertserver

testgo:
	go test ./go/...

.PHONY: testjs
testjs:
	./node_modules/.bin/grunt --no-color test

# Rebuild the third_party Javascript we use, i.e. Flot and our common js.
res/js/perf.js: node_modules Gruntfile.js bower.json res/js/common.js
	./node_modules/.bin/grunt --no-color

node_modules: package.json
	npm install

# Forcibly rebuild third_party JS
.PHONY: js
js:
	./node_modules/.bin/grunt --no-color

# Rebuild the third_party Javascript we use, but w/o minimizing it.
.PHONY: debug-js
debug-js:
	./node_modules/.bin/grunt --no-color debug-js

.PHONY: lint
lint:
	grunt --no-color jshint:main

all: alertserver
