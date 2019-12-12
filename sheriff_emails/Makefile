.PHONY: sheriff_emails
sheriff_emails:
	go install -v ./go/sheriff_emails/...

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: push
push:
	pushk sheriff_emails
