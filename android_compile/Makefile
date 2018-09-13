.PHONY: android_compile
android_compile:
	go install -v ./go/...

.PHONY: android_compile_release
android_compile_release: android_compile
	CGO_ENABLED=0 GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: android_compile_push
android_compile_push: android_compile skia-corp
	pushk android_compile

include ../make/clusters.mk
