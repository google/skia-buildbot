.PHONY: sheriff_emails
sheriff_emails:
	go install -v ./go/sheriff_emails/...

.PHONY: release
release:
	GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: push
push:
	pushk --cluster=skia-public sheriff_emails

include ../make/clusters.mk
