include ../make/bazel.mk

.PHONY: build test release_auth_push

build:
	go install  ./go/...

test:
	go test ./go/...

release_auth_proxy:
	$(BAZEL) run --config=mayberemote //kube/cmd/auth-proxy:push_auth-proxy-container

release_basealpine:
	docker pull alpine:3.9
	./build_basealpine_release

include kube.mk
