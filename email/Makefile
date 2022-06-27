include ../make/bazel.mk

run-local-instance:
	go install ./cmd/...
	emailservice --local

send-from-local-instance:
	curl --include --data-binary @testdata/email.eml -X POST http://localhost:8000/send

.PHONY: release
release:
	$(BAZEL) run //email:push_emailservice

.PHONY: push
push:
	$(BAZEL) run //email:pushk_emailservice
