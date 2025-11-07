include ../make/bazel.mk

.PHONY: trybot_updater
trybot_updater:
	go install -v ./go/trybot-updater/...

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //trybot_updater:push_trybot_updater_container
