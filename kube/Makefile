include ../make/bazel.mk

.PHONY: build test pushk release_auth_push

build:
	go install  ./go/...

test:
	go test ./go/...

release_auth_proxy:
	$(BAZEL) run --config=mayberemote //kube/cmd/auth-proxy:push_auth-proxy-container

release_basealpine:
	docker pull alpine:3.9
	./build_basealpine_release

push: pushk_authproxy pushk_authproxy

pushk_authproxy: release_auth_proxy
	$(BAZEL) run --config=mayberemote //kube/cmd/auth-proxy:pushk_auth-proxy-container

pushk_oauth2redirect:
	$(BAZEL) run --config=mayberemote //kube:pushk_oauth2redirect

include kube.mk
