build: res/css/perf.css res/js/perf.js ingest
	GOPATH=$(GOPATH):`pwd` go build -o perf -v server

tile:
	GOPATH=$(GOPATH):`pwd` go build -v tilebuilder

test:
	GOPATH=$(GOPATH):`pwd` go test kmeans ctrace filetilestore gitinfo
	./node_modules/.bin/grunt --no-color test

verbosetests:
	GOPATH=$(GOPATH):`pwd` go test kmeans ctrace -v

ingest: src/ingest/ingest.go src/ingest/ingest_main.go
	GOPATH=$(GOPATH):`pwd` go build src/ingest/ingest.go src/ingest/ingest_main.go

logs:
	GOPATH=$(GOPATH):`pwd` go build -v logserver

tool: src/ingest/ingest.go
	GOPATH=$(GOPATH):`pwd` go build -v tiletool

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
all: build tile ingest tool
