include ../make/bazel.mk

.PHONY: build test pushk release_configmap_reload release_auth_push

build:
	go install  ./go/...

test:
	go test ./go/...

release_configmap_reload:
	$(BAZEL) run //kube:configmap_reload_container

release_auth_proxy:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/auth-proxy
	./build_auth_proxy_release

release_basealpine:
	docker pull alpine:3.9
	./build_basealpine_release

push: release_auth_proxy release_configmap_reload
	pushk auth-proxy configmap-reload

pushk_authproxy: release_auth_proxy
	pushk auth-proxy

include kube.mk
