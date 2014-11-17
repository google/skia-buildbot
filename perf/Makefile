.PHONY: build
build: res/css/perf.css res/js/perf.js res/vul/elements.html
	go install -v ./go/skiaperf

.PHONY: ver
ver:    H=$(shell echo `git log -n 1 --format=format:%H`)
ver:
	S=$(shell echo $H | cut -c1-5); \
	sed --in-place "s/\(<span id=\"version\">\)\(.*\)\(<\/span>\)/\1<a href=\"https:\/\/skia.googlesource.com\/buildbot\/+\/$H\" target=\"_blank\">Ver:$$S<\/a>\3/" templates/titlebar.html
	git update-index --assume-unchanged templates/titlebar.html

.PHONY: release
release: ver build
	echo

.PHONY: test
test: testgo testjs
	echo

.PHONY: testgo
testgo:
	go test ./go/...

.PHONY: testjs
testjs:
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

.PHONY: perf_migratedb
perf_migratedb:
	go install -v ./go/perf_migratedb

# Rebuild the third_party Javascript we use, i.e. Flot and our common js.
res/js/perf.js: node_modules Gruntfile.js bower.json res/js/common.js
	./node_modules/.bin/grunt --no-color

# Build serving CSS from input CSS.
res/css/perf.css: node_modules res/css/main.css Gruntfile.js bower.json
	./node_modules/.bin/grunt --no-color css

res/vul/elements.html: res/imp/*.html elements.html
	./node_modules/.bin/grunt --no-color mkdir vulcanize

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
all: build ingest tool logs perf_migratedb

.PHONY: tags
tags:
	rm TAGS
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs etags --append
