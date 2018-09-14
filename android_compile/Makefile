.PHONY: android_compile
android_compile:
	go install -v ./go/...

.PHONY: release
release:
	GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: push
push: skia-corp
	pushk --cluster=skia-corp android_compile

include ../make/clusters.mk
