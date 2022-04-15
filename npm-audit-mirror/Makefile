include ../make/bazel.mk
include ../kube/kube.mk

generate:
	go generate ./go/...

testgo:
	go test ./go/...

build: generate
	$(BAZEL) build //npm-audit-mirror/...
