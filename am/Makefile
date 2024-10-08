include ../make/bazel.mk

generate:
	go generate ./go/...

testgo:
	go test ./go/...


# ALERT-MANAGER TARGETS

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/email-chooser-sk:demo_page_server"

run-local-instance-am: build-am
	# Run "bazel build //am/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/am/go/alert-manager/alert-manager_/alert-manager --local --resources_dir ../_bazel_bin/am/pages/development --namespace=am-localhost-jcgregorio
testjs:
	$(BAZEL) test --config=mayberemote modules/...

build-am: generate
	$(BAZEL) build --config=mayberemote //am/pages/...
	$(BAZEL) build --config=mayberemote //am/go/alert-manager:alert-manager

release-am: build-am
	$(BAZEL) run --config=mayberemote //am:push_alert-manager

push-am: release-am
	$(BAZEL) run --config=mayberemote //am:pushk_alert-manager


# ALERT-TO-PUBSUB TARGETS

build-atp:
	$(BAZEL) build --config=mayberemote //am:alert-to-pubsub

release-atp:
	$(BAZEL) run --config=mayberemote //am:push_alert-to-pubsub

push-atp:
	$(BAZEL) run --config=mayberemote //am:pushk_alert-to-pubsub

VERSION := $(shell ../bash/release_tag.sh)

build_alert_to_pubsub_ansible:
	rm -rf ./build
	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/alert_to_pubsub_ansible \
	 -ldflags="-X 'main.Version=$(VERSION)' " \
	 ./go/alert-to-pubsub

build_and_upload_alert_to_pubsub_ansible: build_alert_to_pubsub_ansible
	cipd create -pkg-def=alert_to_pubsub_ansible_cipd.yml --tag version:$(VERSION)

	#
	# The instance can be retrieved using the version tag:
	#
	#   cipd install --root=/tmp/sometempdir skia/internal/alert_to_pubsub_ansible version:${VERSION}

release_alert_to_pubsub_ansible: build_and_upload_alert_to_pubsub_ansible
	../bash/ansible-release.sh alert_to_pubsub_ansible $(VERSION)


# ALERT-MANAGER + ALERT-TO-PUBSUB TARGETS

build-all: build-am build-atp

release-all: release-am release-atp

push-all: push-am push-atp
