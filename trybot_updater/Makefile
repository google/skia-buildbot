include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: trybot_updater
trybot_updater:
	go install -v ./go/trybot-updater/...

.PHONY: release
release:
	$(BAZEL) run //trybot_updater:push_trybot_updater_container

.PHONY: push
push: pushk release
	pushk --use-temp-checkout trybot-updater
