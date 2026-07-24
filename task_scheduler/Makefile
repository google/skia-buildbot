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

.PHONY: validate_db
validate_db:
	go install -v ./go/db/validate_db

.PHONY: build-image-fe
build-image-fe:
	$(BAZEL) run --config=mayberemote //task_scheduler:load_task_scheduler_fe_container

.PHONY: run-local-fe
run-local-fe: build-image-fe
	docker run \
		-p 8090:8090 \
		--user $$(id -u):$$(id -g) \
		-v ~/.config/gcloud:/tmp/gcloud \
		-e CLOUDSDK_CONFIG=/tmp/gcloud \
		-e GOOGLE_APPLICATION_CREDENTIALS=/tmp/gcloud/application_default_credentials.json \
		gcr.io/skia-public/task-scheduler-fe:latest \
		/usr/local/bin/task-scheduler-fe \
		--buildbucket-target=skia://task-scheduler \
		--resources-dir=/usr/local/share/task-scheduler-fe/dist \
		--host=localhost \
		--port=:8090 \
		--firestore-instance=production \
		--bigtable-project=skia-public \
		--bigtable-instance=production \
		--repo=https://skia.googlesource.com/skia.git \
		--swarming-server=chromium-swarm.appspot.com/skia:skia.primary/Skia,SkiaCT,SkiaIOS \
		--local
