include ../make/bazel.mk

.PHONY: serve
serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //status/modules/autoroller-status-sk:demo_page_server"

testgo:
	$(BAZEL) test --config=mayberemote //status/go/...

testjs:
	$(BAZEL) test --config=mayberemote //status/modules/...

build:
	$(BAZEL) build --config=mayberemote //status/...

release: build
	$(BAZEL) run --config=mayberemote //status:push_status_container

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --message="$(MESSAGE)" --use-temp-checkout status
