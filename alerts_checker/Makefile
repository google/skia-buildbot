.PHONY: alerts_checker
alerts_checker:
	go install -v ./go/alerts_checker/...

.PHONY: release
release:
	GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: push
push:
	pushk --cluster=skia-public alerts_checker

include ../make/clusters.mk
