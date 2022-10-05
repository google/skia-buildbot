include ../make/bazel.mk

build:
	${BAZEL} build go/... modules/... pages/...

release: build
	${BAZEL} run //scrap:push_scrapexchange_container

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout scrapexchange

testgo:
	go test ./go/...

testjs:
	${BAZEL} test modules/...

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/scrap-exchange-sk:demo_page_server"

run-local-instance: build
	# Run "bazel build //scrap/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/scrap/go/scrapexchange/scrapexchange_/scrapexchange --local --resources_dir ../_bazel_bin/scrap/pages/development --bucket=skia-public-scrap-exchange
