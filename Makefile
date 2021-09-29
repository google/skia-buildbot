include make/bazel.mk

testgo:
	go test -test.short ./go/...

.PHONY: testjs
testjs:
	cd js && $(MAKE) test

.PHONY: sharedgo
sharedgo:
	cd go && $(MAKE) all

#.PHONY: golden
#golden:
#	cd golden && $(MAKE) all

.PHONY: perf
perf:
	cd perf && $(MAKE) all

.PHONY: autoroll
autoroll:
	cd autoroll && $(MAKE) all

.PHONY: cq_watcher
cq_watcher:
	cd cq_watcher && $(MAKE) default

.PHONY: datahopper
datahopper:
	cd datahopper && $(MAKE) all

.PHONY: ct
ct:
	cd ct && $(MAKE) all

.PHONY: ctfe
ctfe:
	cd ct && $(MAKE) ctfe

.PHONY: infra-sk
infra-sk:
	cd infra-sk && $(MAKE) all

.PHONY: push
push:
	cd push && $(MAKE) default

.PHONY: status
status:
	cd status && $(MAKE) all

.PHONY: fuzzer
fuzzer:
	cd fuzzer && $(MAKE) all

.PHONY: skolo
skolo:
	cd skolo && $(MAKE) all

.PHONY: task_scheduler
task_scheduler:
	cd task_scheduler && $(MAKE) all

# This target is invoked by the Infra-PerCommit-Build tryjob.
.PHONY: all
all: infra-sk autoroll datahopper perf sharedgo ct ctfe cq_watcher status task_scheduler build-frontend-ci

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append

.PHONY: buildall
buildall:
	go build ./...

# Docker image used to run Puppeteer tests (Webpack build).
PUPPETEER_TESTS_DOCKER_IMG=gcr.io/skia-public/rbe-container-skia-infra:2021-04-08T00_09_40Z-lovisolo-2482ab0-clean

# This is invoked from Infra-PerCommit-Puppeteer.
.PHONY: puppeteer-tests
puppeteer-tests:
	# Pull the WASM binaries needed by the debugger-app Webpack build.
	cd debugger-app && $(MAKE) wasm_libs

	docker run --interactive --rm \
		--mount type=bind,source=`pwd`,target=/src \
		--mount type=bind,source=`pwd`/puppeteer-tests/output,target=/out \
		$(PUPPETEER_TESTS_DOCKER_IMG) \
		/src/puppeteer-tests/docker/run-tests.sh

# Front-end code will be built by the Infra-PerCommit-Build tryjob.
#
# All apps with a webpack.config.ts file should be included here.
.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	cd am && $(MAKE) build-frontend-ci
	cd autoroll && $(MAKE) build-frontend-ci
	cd bugs-central && $(MAKE) build-frontend-ci
	cd ct && $(MAKE) build-frontend-ci
	cd debugger-app && $(MAKE) build-frontend-ci
	cd demos && $(MAKE) build-frontend-ci
	cd fiddlek && $(MAKE) build-frontend-ci
	cd hashtag && $(MAKE) build-frontend-ci
	cd infra-sk && $(MAKE) build-frontend-ci
	cd jsfiddle && $(MAKE) build-frontend-ci
	cd leasing && $(MAKE) build-frontend-ci
	cd new_element && $(MAKE) build-frontend-ci
	cd particles && $(MAKE) build-frontend-ci
	cd perf && $(MAKE) build-frontend-ci
	cd power && $(MAKE) build-frontend-ci
	cd pulld && $(MAKE) build-frontend-ci
	cd push && $(MAKE) build-frontend-ci
	cd scrap && $(MAKE) build-frontend-ci
	cd skottie && $(MAKE) build-frontend-ci
	cd status && $(MAKE) build-frontend-ci
	cd task_driver && $(MAKE) build-frontend-ci
	cd task_scheduler && $(MAKE) build-frontend-ci
	cd tree_status && $(MAKE) build-frontend-ci

# Front-end tests will be included in the Infra-PerCommit-Medium tryjob.
#
# All apps with a karma.conf.ts file should be included here.
.PHONY: test-frontend-ci
test-frontend-ci: npm-ci
	cd am && $(MAKE) test-frontend-ci
	cd ct && $(MAKE) test-frontend-ci
	cd debugger-app && $(MAKE) test-frontend-ci
	cd demos && $(MAKE) test-frontend-ci
	cd fiddlek && $(MAKE) test-frontend-ci
	cd infra-sk && $(MAKE) test-frontend-ci
	cd new_element && $(MAKE) test-frontend-ci
	cd perf && $(MAKE) test-frontend-ci
	cd puppeteer-tests && $(MAKE) test-frontend-ci
	cd push && $(MAKE) test-frontend-ci
	cd scrap && $(MAKE) test-frontend-ci
	cd shaders && $(MAKE) test-frontend-ci
	cd status && $(MAKE) test-frontend-ci
	cd task_scheduler && $(MAKE) test-frontend-ci

.PHONY: update-go-bazel-files
update-go-bazel-files:
	$(BAZEL) run //:gazelle -- update ./

.PHONY: update-go-bazel-deps
update-go-bazel-deps:
	$(BAZEL) run //:gazelle -- update-repos -from_file=go.mod -to_macro=go_repositories.bzl%go_repositories

.PHONY: gazelle
gazelle: update-go-bazel-deps update-go-bazel-files

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
