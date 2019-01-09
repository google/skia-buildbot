include ../go/skiaversion/skiaversion.mk
include ../kube/kube.mk

.PHONY: build
build: skiaversion
	GOOS=linux go install -a ./go/periodic-trigger

.PHONY: push
push: build pushk
	./build_release
	pushk --message="$(MESSAGE)" --cluster=skia-public periodic-trigger
