include ../kube/kube.mk

default:
	go install -v ./go/...

release: default
	CGO_ENABLED=0 GOOS=linux go install -a ./go/cq_watcher
	./build_docker_release

push: pushk release
	pushk --use-temp-checkout cq_watcher
