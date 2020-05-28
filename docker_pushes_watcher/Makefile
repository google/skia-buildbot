include ../kube/kube.mk

.PHONY: build test pushk release push

build:
	go install  ./go/...

test:
	go test ./go/...

release: build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/docker_pushes_watcher ./../kube/go/pushk
	./build_docker_release

push: pushk release
	pushk --use-temp-checkout docker_pushes_watcher
