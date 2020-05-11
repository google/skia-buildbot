include ../kube/kube.mk

.PHONY: datahopper
datahopper:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/datahopper

.PHONY: all
all: datahopper

.PHONY: release
release: datahopper
	./build_release

.PHONY: push
push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout datahopper
