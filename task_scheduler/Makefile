include ../make/bazel.mk
include ../kube/kube.mk

# First create a list of all the modules that ./go/rpc depends upon.
rpcDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' ./go/rpc)

# Then use that list to find all the Go files that make up those dependent modules.
rpcDependentFiles := ./go/rpc/rpc.proto $(shell echo $(rpcDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}')

rpcOutputFiles := ./go/rpc/rpc.pb.go ./go/rpc/rpc.twirp.go ./modules/rpc/rpc.ts ./modules/rpc/twirp.ts

$(rpcOutputFiles): $(rpcDependentFiles)
	go generate ./go/rpc

.PHONY: serve
serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //infra-sk/modules/task-driver-sk:demo_page_server"

.PHONY: all
all: task-scheduler-be task-scheduler-fe task-scheduler-jc validate_db

.PHONY: task-scheduler-be
task-scheduler-be:
	$(BAZEL) build --config=mayberemote //task_scheduler/go/task-scheduler-be/...

.PHONY: task-scheduler-fe
task-scheduler-fe:
	$(BAZEL) build --config=mayberemote //task_scheduler/...

.PHONY: task-scheduler-jc
task-scheduler-jc:
	$(BAZEL) build --config=mayberemote //task_scheduler/go/task-scheduler-jc/...

.PHONY: release-be
release-be: task-scheduler-be build_base_cipd_release
	$(BAZEL) run --config=mayberemote //task_scheduler:push_task_scheduler_be_container

.PHONY: release-fe
release-fe: task-scheduler-fe
	$(BAZEL) run --config=mayberemote //task_scheduler:push_task_scheduler_fe_container

.PHONY: release-jc
release-jc: task-scheduler-jc
	$(BAZEL) run --config=mayberemote //task_scheduler:push_task_scheduler_jc_container

.PHONY: push-be
push-be: release-be
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout task-scheduler-be

.PHONY: push-fe
push-fe: release-fe
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout task-scheduler-fe

.PHONY: push-jc
push-jc: release-jc
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout task-scheduler-jc

.PHONY: validate_db
validate_db:
	go install -v ./go/db/validate_db
