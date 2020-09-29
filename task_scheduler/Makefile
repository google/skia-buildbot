VULCANIZE1=true

BOWER_DIR=third_party/bower_components

# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
	third_party/bower_components/webcomponentsjs/webcomponents.min.js \
	$(BOWER_DIR)/d3/d3.js \
	../res/js/common.js

include ../kube/kube.mk

# First create a list of all the modules that ./go/rpc depends upon.
rpcDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' ./go/rpc)

# Then use that list to find all the Go files that make up those dependent modules.
rpcDependentFiles := ./go/rpc/rpc.proto $(shell echo $(rpcDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}')

rpcOutputFiles := ./go/rpc/rpc.pb.go ./go/rpc/rpc.twirp.go ./modules/rpc/rpc.ts ./modules/rpc/twirp.ts

$(rpcOutputFiles): $(rpcDependentFiles)
	go generate ./go/rpc

./package-lock.json: package.json
	npm install

./node_modules/lastupdate-webpack: ./package-lock.json
	npm ci
	touch ./node_modules/lastupdate-webpack

pages := $(shell find ./pages -type f)
modules := $(shell find ./modules -type f)
frontendInputFiles := $(pages) $(modules) $(rpcOutputFiles) ./node_modules/lastupdate-webpack
pageBaseNames := $(sort $(basename $(notdir $(pages))))
distNames := $(addsuffix -bundle.js,$(pageBaseNames)) $(addsuffix -bundle.css,$(pageBaseNames)) $(addsuffix .html,$(pageBaseNames))
dist := $(addprefix ./dist/,$(distNames))

$(dist): $(frontendInputFiles)
	npx webpack --mode=production

.PHONY: serve
serve: $(dist)
	npx webpack-dev-server --mode=development --watch-poll

.PHONY: test-frontend
test-frontend: ./node_modules/lastupdate-webpack
	npx karma start

.PHONY: test-frontend-ci
test-frontend-ci: ./node_modules/lastupdate-webpack
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: puppeteer-test
puppeteer-test: ./node_modules/lastupdate-webpack
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -v -r ts-node/register ./modules/**/*_puppeteer_test.ts

.PHONY: all
all: task-scheduler-be task-scheduler-fe task-scheduler-jc validate_db

.PHONY: task-scheduler-be
task-scheduler-be:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-be

.PHONY: task-scheduler-fe
task-scheduler-fe: $(dist) core_js elements_html
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-fe

.PHONY: task-scheduler-jc
task-scheduler-jc:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-jc

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

.PHONY: task-scheduler-fe-debug
task-scheduler-fe-debug: debug
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-fe

.PHONY: release-be
release-be: task-scheduler-be build_base_cipd_release
	./build_be_release

.PHONY: release-fe
release-fe: task-scheduler-fe
	npx webpack --mode=production
	./build_fe_release

.PHONY: release-jc
release-jc: task-scheduler-jc
	./build_jc_release

.PHONY: push-be
push-be: release-be
	pushk --message="$(MESSAGE)" --use-temp-checkout task-scheduler-be

.PHONY: push-fe
push-fe: release-fe
	pushk --message="$(MESSAGE)" --use-temp-checkout task-scheduler-fe

.PHONY: push-jc
push-jc: release-jc
	pushk --message="$(MESSAGE)" --use-temp-checkout task-scheduler-jc

.PHONY: validate_db
validate_db:
	go install -v ./go/db/validate_db

include ../webtools/webtools.mk
