include ../go/skiaversion/skiaversion.mk

.PHONY: build
build: skiaversion
	GOOS=linux go install -a ./go/periodic-trigger

.PHONY: push
push: build
	./build_release
	go install -v ../push/go/pushcli
	pushk --message="$(MESSAGE)" --cluster=skia-public periodic-trigger
