.PHONY: build
build: androidingest query_buildapi

.PHONY: query_buildapi
query_buildapi:
	go install -v ./go/query_buildapi

.PHONY: androidingest
androidingest:
	go install -v ./go/androidingest

.PHONY: testgo
testgo:
	go test ./go/...

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/androidingest
	./images/android_ingest/release

.PHONY: push
pushk: release
	pushk androidingest

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

.PHONY: all
all: build

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append
