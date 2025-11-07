include ../make/bazel.mk

run-local-instance:
	go install ./cmd/...
	emailservice --local

send-from-local-instance:
	curl --include --data-binary @testdata/email.eml -X POST http://localhost:8000/send

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //email:push_emailservice
