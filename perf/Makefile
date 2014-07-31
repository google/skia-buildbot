.PHONY: build
build: res/css/perf.css res/js/perf.js
	go install -v ./go/skiaperf

.PHONY: tilebuilder
tilebuilder:
	go install -v ./go/tilebuilder

.PHONY: test
test:
	go test ./go/kmeans ./go/ctrace ./go/filetilestore ./go/gitinfo
	./node_modules/.bin/grunt --no-color test

.PHONY: ingest
ingest:
	go install -v ./go/ingest

.PHONY: logs
logs:
	go install -v ./go/logserver

.PHONY: tool
tool:
	go install -v ./go/tiletool

# Rebuild the third_party Javascript we use, i.e. Flot.
res/js/perf.js: node_modules Gruntfile.js bower.json
	./node_modules/.bin/grunt --no-color

# Build serving CSS from input CSS.
res/css/perf.css: node_modules res/css/main.css Gruntfile.js bower.json
	./node_modules/.bin/grunt --no-color css

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

.PHONY: all
all: build tilebuilder ingest tool logs
