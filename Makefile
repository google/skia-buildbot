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

.PHONY: puppeteer-tests-npm-deps
puppeteer-tests-npm-deps:
	cd puppeteer-tests && $(MAKE) all

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
all: puppeteer-tests-npm-deps infra-sk autoroll datahopper perf sharedgo ct ctfe cq_watcher status task_scheduler build-frontend-ci

.PHONY: tags
tags:
	-rm tags
	find . -name "*.go" -print -or -name "*.js" -or -name "*.html" | xargs ctags --append

.PHONY: buildall
buildall:
	go build ./...

.PHONY: puppeteer-tests
puppeteer-tests:
	# Pull the WASM binaries needed by the debugger-app Webpack build.
	cd debugger-app && $(MAKE) wasm_libs

	docker run --interactive --rm \
		--mount type=bind,source=`pwd`,target=/src \
		--mount type=bind,source=`pwd`/puppeteer-tests/output,target=/out \
		gcr.io/skia-public/puppeteer-tests:latest \
		/src/puppeteer-tests/docker/run-tests.sh

# Front-end code will be built by the Infra-PerCommit-Build tryjob.
#
# All apps with a webpack.config.ts file should be included here.
.PHONY: build-frontend-ci
build-frontend-ci:
	# Generate the //puppeteer-tests/node_modules directory. Some targets will not compile without it.
	cd puppeteer-tests && npm ci

	# infra-sk needs to be built first because this pulls its NPM dependencies
	# with "npm ci", which are needed by other apps.
	cd infra-sk && $(MAKE) build-frontend-ci

	# Other apps can be built in alphabetical order.
	cd am && $(MAKE) build-frontend-ci
	cd ct && $(MAKE) build-frontend-ci
	cd debugger-app && $(MAKE) build-frontend-ci
	cd demos && $(MAKE) build-frontend-ci
	cd golden && $(MAKE) build-frontend-ci
	cd hashtag && $(MAKE) build-frontend-ci
	cd jsfiddle && $(MAKE) build-frontend-ci
	cd leasing && $(MAKE) build-frontend-ci
	cd machine && $(MAKE) build-frontend-ci
	cd new_element && $(MAKE) build-frontend-ci
	cd particles && $(MAKE) build-frontend-ci
	cd perf && $(MAKE) build-frontend-ci
	cd power && $(MAKE) build-frontend-ci
	cd pulld && $(MAKE) build-frontend-ci
	cd push && $(MAKE) build-frontend-ci
	cd skottie && $(MAKE) build-frontend-ci
	cd status && $(MAKE) build-frontend-ci
	cd task_driver && $(MAKE) build-frontend-ci
	cd tree_status && $(MAKE) build-frontend-ci

# Front-end tests will be included in the Infra-PerCommit-Medium tryjob.
#
# All apps with a karma.conf.ts file should be included here.
.PHONY: test-frontend-ci
test-frontend-ci:
	# Generate the //puppeteer-tests/node_modules directory. Some targets will not compile without it.
	cd puppeteer-tests && npm ci

	# infra-sk needs to be tested first because this pulls its NPM dependencies
	# with "npm ci", which are needed by other apps.
	cd infra-sk && $(MAKE) test-frontend-ci

	# Other apps can be tested in alphabetical order.
	cd am && $(MAKE) test-frontend-ci
	cd ct && $(MAKE) test-frontend-ci
	cd debugger-app && $(MAKE) test-frontend-ci
	cd demos && $(MAKE) test-frontend-ci
	cd golden && $(MAKE) test-frontend-ci
	cd machine && $(MAKE) test-frontend-ci
	cd new_element && $(MAKE) test-frontend-ci
	cd perf && $(MAKE) test-frontend-ci
	cd push && $(MAKE) test-frontend-ci
	cd fiddlek && $(MAKE) test-frontend-ci

.PHONY: update-go-bazel-files
update-go-bazel-files:
	bazel run //:gazelle ./go/

.PHONY: update-machine-bazel-files
update-machine-bazel-files:
	bazel run //:gazelle ./machine/

.PHONE: update-go-bazel-deps
update-go-bazel-deps:
	bazel run //:gazelle -- update-repos -from_file=go.mod