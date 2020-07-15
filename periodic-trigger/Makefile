include ../kube/kube.mk

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/periodic-trigger

.PHONY: release
release: build
	./build_release

.PHONY: push
push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout periodic-trigger
