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
	cd status && $(MAKE) test-frontend-ci

# Directories under //go that can be built using Gazelle-generated BUILD files. Eventually this will
# be replaced with just ./go.
#
# The below list of directories (minus those that have been manually removed) can be regeneated
# using the following command:
#
#   $ ls go | sed -E "s/(.*)/.\/go\/\1 \\\/" | grep -v Makefile
GAZELLE_GO_DIRS=\
	./go/alerts \
	./go/allowed \
	./go/androidbuild \
	./go/androidbuildinternal \
	./go/android_hashlookup \
	./go/android_skia_checkout \
	./go/atomic_miss_cache \
	./go/auditlog \
	./go/auth \
	./go/buildskia \
	./go/baseapp \
	./go/benchmarks \
	./go/bt \
	./go/calc \
	./go/chatbot \
	./go/chrome_branch \
	./go/cipd \
	./go/cleanup \
	./go/codesearch \
	./go/comment \
	./go/common \
	./go/config \
	./go/counters \
	./go/dataproc \
	./go/deepequal \
	./go/depot_tools \
	./go/docker \
	./go/ds \
	./go/email \
	./go/exec \
	./go/executil \
	./go/fileutil \
	./go/firestore \
	./go/gce \
	./go/gcr \
	./go/gcs \
	./go/git \
	./go/gitauth \
	./go/github \
	./go/gitiles \
	./go/gitstore \
	./go/go_install \
	./go/httputils \
	./go/human \
	./go/imports \
	./go/isolate \
	./go/issues \
	./go/jsonutils \
	./go/kube \
	./go/login \
	./go/metadata \
	./go/metrics2 \
	./go/mockhttpclient \
	./go/monorail \
	./go/notifier \
	./go/packages \
	./go/paramreducer \
	./go/paramtools \
	./go/periodic \
	./go/query \
	./go/recipe_cfg \
	./go/repo_root \
	./go/ring \
	./go/rotations \
	./go/rtcache \
	./go/skerr \
	./go/skiaversion \
	./go/sklog \
	./go/sktest \
	./go/state_machine \
	./go/systemd \
	./go/tar \
	./go/taskname \
	./go/test2json \
	./go/testutils \
	./go/timeout \
	./go/timer \
	./go/travisci \
	./go/trie \
	./go/twirp_auth \
	./go/untar \
	./go/urfavecli \
	./go/util \
	./go/vcsinfo \
	./go/vec32 \
	./go/vfs \
	./go/webhook \
	./go/workerpool

# Directories under //go that fail to compile using Gazelle-generated BUILD files.
#
# These directories should be fixed one by one and moved to the $GAZELLE_GO_DIRS list above until
# there are none left.
#
# 	./go/autoroll
# 	./go/buildbucket
# 	./go/cas
# 	./go/cq
# 	./go/gerrit
# 	./go/luciauth
# 	./go/swarming
#		./go/supported_branches

# Directories with Go code that can compile using Gazelle-generated BUILD files.
GAZELLE_DIRS=\
	./bazel \
	$(GAZELLE_GO_DIRS) \
	./machine

.PHONY: update-go-bazel-files
update-go-bazel-files:
	bazel run //:gazelle -- $(GAZELLE_DIRS)

.PHONY: update-go-bazel-deps
update-go-bazel-deps:
	bazel run //:gazelle -- update-repos -from_file=go.mod -to_macro=go_repositories.bzl%go_repositories

# Known good Bazel build targets. Eventually this should be replaced with "bazel build all".
BAZEL_BUILD_TARGETS=\
	//bazel/... \
	//go/... \
	//infra-sk/... \
	//machine/... \
	//puppeteer-tests/... \

# Known good Bazel test targets. Eventually this should be replaced with "bazel test all".
BAZEL_TEST_TARGETS=\
	//bazel/... \
	//infra-sk/... \
	//machine/modules/...\
	//puppeteer-tests/... \

.PHONY: bazel-build
bazel-build:
	bazel build $(BAZEL_BUILD_TARGETS)

.PHONY: bazel-test
bazel-test:
	bazel test $(BAZEL_TEST_TARGETS)

.PHONY: bazel-test-nocache
bazel-test-nocache:
	bazel test --cache_test_results=no $(BAZEL_TEST_TARGETS)

.PHONY: bazel-build-rbe
bazel-build-rbe:
	bazel build --config=remote $(BAZEL_BUILD_TARGETS)

.PHONY: bazel-test-rbe
bazel-test-rbe:
	bazel test --config=remote $(BAZEL_TEST_TARGETS)

.PHONY: bazel-test-rbe-nocache
bazel-test-rbe-nocache:
	bazel test --config=remote --cache_test_results=no $(BAZEL_TEST_TARGETS)
