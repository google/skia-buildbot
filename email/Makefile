include ../make/bazel.mk

run-local-instance:
	go install ./cmd/...
	emailservice --local

send-from-local-instance:
	curl --include --data-binary @testdata/email.eml -X POST http://localhost:8000/send

.PHONY: release
release:
	$(BAZEL) run --config=mayberemote //email:push_emailservice

.PHONY: push
push:
	$(BAZEL) run --config=mayberemote //email:pushk_emailservice

rotate-skia-corp-service-account-key:
	../kube/secrets/rotate-keys-for-skia-corp-sa.sh skia-public skia-emailservice deployment/emailservice
