include ../make/bazel.mk

.PHONY: trybot_updater
trybot_updater:
	go install -v ./go/trybot-updater/...

.PHONY: release
release:
	$(BAZEL) run //trybot_updater:push_trybot_updater_container

.PHONY: push
push: release
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout trybot-updater
