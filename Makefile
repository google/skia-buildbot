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

.PHONY: gazelle
gazelle:
	$(BAZEL) run --config=mayberemote //:gazelle -- update ./

# Run this if we need to update our JS/TS packages
.PHONY: update-npm
update-npm:
	$(BAZEL) run -- @pnpm//:pnpm --dir $(PWD) install --lockfile-only

.PHONY: buildifier
buildifier:
	$(BAZEL) run --config=mayberemote //:buildifier

.PHONY: gofmt
gofmt:
	$(BAZEL) run --config=mayberemote //:gofmt -- -s -w .

node_modules: package-lock.json
	$(BAZEL) run --config=mayberemote //:npm -- ci

# We add node_modules as a dependency because npx looks for prettier in that directory.
.PHONY: prettier
prettier: node_modules
	$(BAZEL) run --config=mayberemote //:npx -- prettier --write . --ignore-unknown

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
	-npx eslint --quiet --fix .

.PHONY: errcheck
errcheck:
	$(BAZEL) run //:errcheck -- -ignore :Close go.skia.org/infra/...

.PHONY: mocks
mocks:
	$(BAZEL) run //:mockery

include make/npm.mk
