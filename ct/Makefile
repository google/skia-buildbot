include ../make/bazel.mk

generate:
	go generate ./go/...

testgo:
	$(BAZEL) test go/...

testpy:
	$(BAZEL) test py/...

testjs:
	${BAZEL} test modules/...

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/ct-scaffold-sk:demo_page_server"

run-local-instance: build
	# Run "bazel build //ct/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	../_bazel_bin/ct/go/ctfe/ctfe_/ctfe --local --resources_dir ../_bazel_bin/ct/pages/development --namespace=cluster-telemetry-staging

build: generate
	$(BAZEL) build go/ctfe/... modules/... pages/...

release: build
	$(BAZEL) run //ct:push_ctfe_container

push: release
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout ctfe

