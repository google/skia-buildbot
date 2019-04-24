.PHONY: k8s_checker
k8s_checker:
	go install -v ./go/k8s_checker/...

.PHONY: release
release:
	GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: push
push:
	pushk --cluster=skia-public k8s_checker
	pushk --cluster=skia-corp k8s_checker

include ../make/clusters.mk
