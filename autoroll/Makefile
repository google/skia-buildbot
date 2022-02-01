# This makefile is used to compile Autoroller code, build Docker images
# containing the same, and generate, apply, and push Kubernetes configs
# for the autorollers.

include ../make/bazel.mk
include ../kube/kube.mk

# First create a list of all the modules that ./go/rpc depends upon.
rpcDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' ./go/rpc)

# Then use that list to find all the Go files that make up those dependent modules.
rpcDependentFiles := ./go/rpc/rpc.proto $(shell echo $(rpcDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}')

rpcOutputFiles := ./go/rpc/rpc.pb.go ./go/rpc/rpc.twirp.go ./modules/rpc/rpc.ts ./modules/rpc/twirp.ts

$(rpcOutputFiles): $(rpcDependentFiles)
	go generate ./go/rpc

.PHONY: autoroll-fe
autoroll-fe:
	$(BAZEL) build //autoroll/...

.PHONY: autoroll-be
autoroll-be:
	$(BAZEL) build //autoroll/go/autoroll-be/...

.PHONY: autoroll-google3
autoroll-google3:
	$(BAZEL) build //autoroll/go/autoroll-google3/...

.PHONY: all
all: autoroll-fe autoroll-be autoroll-google3

.PHONY: serve
serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //autoroll/modules/arb-status-sk:demo_page_server"

ROLLER_INTERNAL_CONFIG_DIR = "/tmp/skia-autoroll-internal-config"
USE_INTERNAL_CONFIG_DIR_PROMPT ?= $$(bash -c 'read -s -p "The checkout in $(ROLLER_INTERNAL_CONFIG_DIR) is dirty. Would you still like to use it? (y/n): " ans; echo $$ans')

setup-internal-config-repo:
	if [[ ! -d $(ROLLER_INTERNAL_CONFIG_DIR) ]]; then \
		git clone https://skia.googlesource.com/skia-autoroll-internal-config.git $(ROLLER_INTERNAL_CONFIG_DIR); \
	else \
		git -C $(ROLLER_INTERNAL_CONFIG_DIR) fetch && \
		git -C $(ROLLER_INTERNAL_CONFIG_DIR) diff --quiet origin/main || \
		if [ $$? -ne 0 ]; then \
			if [[ "$(USE_INTERNAL_CONFIG_DIR_PROMPT)" != "y" ]]; then \
				echo "Exiting"; exit 1; \
			fi; \
		fi; \
	fi

# These targets build kubernetes releases.
.PHONY: release-be
release-be: autoroll-be
	$(BAZEL) run //autoroll:push_autoroll_be_container

.PHONY: release-fe
release-fe: autoroll-fe setup-internal-config-repo
	$(BAZEL) run //autoroll:push_autoroll_fe_container
	$(BAZEL) run //autoroll:push_autoroll_fe_internal_container
	# Cleanup the config directory.
	git clean -fxq config/*

.PHONY: release-google3
release-google3: autoroll-google3 setup-internal-config-repo
	$(BAZEL) run //autoroll:push_autoroll_google3_container
	# Cleanup the config directory.
	git clean -fxq config/*

.PHONY: release-all
release-all: release-fe release-be release-google3

# autoroll-pusher is used by the below targets to apply k8s configurations.
.PHONY: autoroll-pusher
autoroll-pusher: kube-conf-gen deployment-dirs
	go install -v ./go/autoroll-pusher

# push-roller is the same as apply-roller, but it also commits and pushes the
# modified kubernetes config files into the git repo, using the commit message
# from the MESSAGE environment variable.
.PHONY: push-roller
push-roller: release-fe release-be autoroll-pusher
	autoroll-pusher --roller="$(ROLLER)" --update-config --update-be-image --update-fe-image --commit-msg="$(MESSAGE)"

# push-be generates a new backend release build and applies it to ALL rollers,
# then commits and pushes the modified kubernetes config files into the git
# repo, using the commit message from the MESSAGE environment variable. Does NOT
# update the roller config files used by the backends.
.PHONY: push-be
push-be: release-be autoroll-pusher
	autoroll-pusher --update-be-image --commit-msg="$(MESSAGE)"

# push-fe generates a new frontend release build, applies it, and commits and
# pushes the modified kubernetes config files into the git repo, using the
# commit message from the MESSAGE environment variable. Does NOT update the
# roller config files used by the frontends.
.PHONY: push-fe
push-fe: release-fe autoroll-pusher
	autoroll-pusher --update-fe-image --commit-msg="$(MESSAGE)"

# push-all generates new frontend and backend release builds, applies them along
# with the current roller config files(s), and commits and pushes the modified
# kubernetes config files into the git repo, using the commit message from the
# MESSAGE environment variable.
.PHONY: push-all
push-all: release-be release-fe autoroll-pusher
	autoroll-pusher --update-config --update-be-image --update-fe-image --commit-msg="$(MESSAGE)"

# validate runs the validate_autoroll_config task driver using the provided
# CONFIG variable, which may be a single config file or a directory containing
# config files.
.PHONY: validate
validate:
	go run ../infra/bots/task_drivers/validate_autoroll_configs/validate_autoroll_configs.go --local --config="$(CONFIG)"

# Run the autoroll-fe locally. This is just for convenience in testing.
.PHONY: run-local-fe-instance
run-local-fe-instance: autoroll-fe setup-internal-config-repo
	# Run "bazel build //autoroll/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/autoroll/go/autoroll-fe/autoroll-fe_/autoroll-fe --local --firestore_instance=staging --config_file=$(ROLLER_INTERNAL_CONFIG_DIR)/skia-public/skia-skiabot-test.cfg --resources_dir ../_bazel_bin/autoroll/pages/development --config_repo=https://skia.googlesource.com/skia-autoroll-internal-config.git --config_gerrit_project=skia-autoroll-internal-config
