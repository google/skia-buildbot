include make/bazel.mk

testgo:
	go test -test.short ./go/...

.PHONY: testjs
testjs:
	cd js && $(MAKE) test

.PHONY: skolo
skolo:
	cd skolo && $(MAKE) all

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append

.PHONY: buildall
buildall:
	go build ./...

.PHONY: update-go-bazel-files
update-go-bazel-files:
	$(BAZEL) run --config=mayberemote //:gazelle -- update ./

.PHONY: update-go-bazel-deps
update-go-bazel-deps:
	$(BAZEL) run --config=mayberemote //:gazelle -- update-repos -from_file=go.mod -to_macro=go_repositories.bzl%go_repositories

.PHONY: gazelle
gazelle: update-go-bazel-deps update-go-bazel-files

.PHONY: buildifier
buildifier:
	$(BAZEL) run --config=mayberemote //:buildifier

.PHONY: gofmt
gofmt:
	$(BAZEL) run --config=mayberemote //:gofmt -- -s -w .

.PHONY: bazel-build
bazel-build:
	$(BAZEL) build //...

.PHONY: bazel-test
bazel-test:
	$(BAZEL) test //...

.PHONY: bazel-test-nocache
bazel-test-nocache:
	$(BAZEL) test --cache_test_results=no //...

.PHONY: bazel-build-rbe
bazel-build-rbe:
	$(BAZEL) build --config=remote //...

.PHONY: bazel-test-rbe
bazel-test-rbe:
	$(BAZEL) test --config=remote //...

.PHONY: bazel-test-rbe-nocache
bazel-test-rbe-nocache:
	$(BAZEL) test --config=remote --cache_test_results=no //...

.PHONY: eslint
eslint:
	-npx eslint --fix .

include make/npm.mk
