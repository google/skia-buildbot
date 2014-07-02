build: res/css/perf.css res/js/perf.js
	GOPATH=$(GOPATH):`pwd` go build -o perf -v server

tile:
	GOPATH=$(GOPATH):`pwd` go build -v tilebuilder


test:
	GOPATH=$(GOPATH):`pwd` go test kmeans ctrace
	./node_modules/.bin/grunt test

verbosetests:
	GOPATH=$(GOPATH):`pwd` go test kmeans ctrace -v

# Rebuild the third_party Javascript we use, i.e. Flot.
res/js/perf.js: node_modules
	./node_modules/.bin/grunt

# Build serving CSS from input CSS.
res/css/perf.css: node_modules res/css/main.css
	./node_modules/.bin/grunt css

node_modules: package.json
	npm install

# Forcibly rebuild third_party JS
.PHONY: js
js:
	./node_modules/.bin/grunt

# Rebuild the third_party Javascript we use, but w/o minimizing it.
.PHONY: debug-js
debug-js:
	./node_modules/.bin/grunt debug-js

