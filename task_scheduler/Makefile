include ../kube/kube.mk

# First create a list of all the modules that ./go/rpc depends upon.
rpcDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' ./go/rpc)

# Then use that list to find all the Go files that make up those dependent modules.
rpcDependentFiles := ./go/rpc/rpc.proto $(shell echo $(rpcDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}')

rpcOutputFiles := ./go/rpc/rpc.pb.go ./go/rpc/rpc.twirp.go ./modules/rpc/rpc.ts ./modules/rpc/twirp.ts

$(rpcOutputFiles): $(rpcDependentFiles)
	go generate ./go/rpc

pages := $(shell find ./pages -type f)
modules := $(shell find ./modules -type f)
frontendInputFiles := $(pages) $(modules) $(rpcOutputFiles) ./res/img/favicon.ico npm-ci
pageBaseNames := $(sort $(basename $(notdir $(pages))))
distNames := $(addsuffix -bundle.js,$(pageBaseNames)) $(addsuffix -bundle.css,$(pageBaseNames)) $(addsuffix .html,$(pageBaseNames)) favicon.ico
dist := $(addprefix ./dist/,$(distNames))

$(dist): $(frontendInputFiles)
	npx webpack --mode=production

.PHONY: serve
serve: $(dist)
	npx webpack-dev-server --mode=development --watch-poll

.PHONY: test-frontend
test-frontend: npm-ci
	npx karma start

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: puppeteer-test
puppeteer-test: npm-ci
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -v -r ts-node/register ./modules/**/*_puppeteer_test.ts

.PHONY: all
all: task-scheduler-be task-scheduler-fe task-scheduler-jc validate_db

.PHONY: task-scheduler-be
task-scheduler-be:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-be

.PHONY: task-scheduler-fe
task-scheduler-fe: $(dist)
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-fe

.PHONY: task-scheduler-jc
task-scheduler-jc:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/task-scheduler-jc

.PHONY: release-be
release-be: task-scheduler-be build_base_cipd_release
	./build_be_release

.PHONY: release-fe
release-fe: task-scheduler-fe npm-ci
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

include ../make/npm.mk
