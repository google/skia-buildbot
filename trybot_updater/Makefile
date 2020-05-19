include ../kube/kube.mk

.PHONY: trybot_updater
trybot_updater:
	go install -v ./go/trybot-updater/...

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/...
	./build_release

.PHONY: push
push: pushk release
	pushk --use-temp-checkout trybot-updater
