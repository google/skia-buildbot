# This makefile is used to compile Autoroller code, build Docker images
# containing the same, and generate, apply, and push Kubernetes configs
# for the autorollers.

include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: autoroll-fe
autoroll-fe:
	$(BAZEL) build --config=mayberemote //autoroll/...

.PHONY: autoroll-be
autoroll-be:
	$(BAZEL) build --config=mayberemote //autoroll/go/autoroll-be/...

.PHONY: autoroll-google3
autoroll-google3:
	$(BAZEL) build --config=mayberemote //autoroll/go/autoroll-google3/...

.PHONY: all
all: autoroll-fe autoroll-be autoroll-google3

.PHONY: serve
serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //autoroll/modules/arb-status-sk:demo_page_server"

# These targets build kubernetes releases.
.PHONY: release-be
release-be: autoroll-be
	$(BAZEL) run --config=mayberemote //autoroll:push_autoroll_be_container

.PHONY: release-fe
release-fe: autoroll-fe
	$(BAZEL) run --config=mayberemote //autoroll:push_autoroll_fe_container

.PHONY: release-google3
release-google3: autoroll-google3
	$(BAZEL) run --config=mayberemote //autoroll:push_autoroll_google3_container
	# Cleanup the config directory.
	git clean -fxq config/*

.PHONY: release-all
release-all: release-fe release-be release-google3

# validate runs the validate_autoroll_config task driver using the provided
# CONFIG variable, which may be a single config file or a directory containing
# config files.
.PHONY: validate
validate:
	go run ../infra/bots/task_drivers/validate_autoroll_configs/validate_autoroll_configs.go --local --config="$(CONFIG)"

# Run the autoroll-fe locally. This is just for convenience in testing.
.PHONY: run-local-fe-instance
run-local-fe-instance: autoroll-fe
	# Run "bazel build //autoroll/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/autoroll/go/autoroll-fe/autoroll-fe_/autoroll-fe --local --firestore_instance=production --config_repo=https://skia.googlesource.com/skia-autoroll-internal-config.git --resources_dir ../_bazel_bin/autoroll/pages/development --config_repo=https://skia.googlesource.com/skia-autoroll-internal-config.git --config_gerrit_project=skia-autoroll-internal-config
