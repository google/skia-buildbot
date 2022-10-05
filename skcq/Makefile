include ../make/bazel.mk

generate:
	go generate ./go/...

testgo:
	go test ./go/...


# FRONTEND TARGETS

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/skcq-scaffold-sk:demo_page_server"

run-local-instance-fe: build-fe
	# Run "bazel build //skcq/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/skcq/go/skcq-fe/skcq-fe_/skcq-fe --local --resources_dir ../_bazel_bin/skcq/pages/development --fs_namespace=staging

testjs:
	$(BAZEL) test --config=mayberemote modules/...

build-fe: generate
	$(BAZEL) build --config=mayberemote go/skcq-fe/... modules/... pages/...

release-fe: build-fe
	$(BAZEL) run --config=mayberemote //skcq:push_skcq_fe_container

push-fe: release-fe
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout skcq-fe


# BACKEND TARGETS

run-local-instance-be: build-be
	../_bazel_bin/skcq/go/skcq-be/skcq-be_/skcq-be --local --resources_dir ../_bazel_bin/skcq/pages/development --fs_namespace=staging --allowed_repo=skiabot-playground

build-be: generate
	$(BAZEL) build --config=mayberemote go/skcq-be/...

release-be: build-be
	$(BAZEL) run --config=mayberemote //skcq:push_skcq_be_container

push-be: release-be
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout skcq-be


# FRONTEND+BACKEND TARGETS

build-all: build-fe build-be

release-all: release-fe release-be

push-all: push-fe push-be
