.PHONY: k8s-checker
k8s_checker:
	go install -v ./go/k8s-checker/...

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: push
push: release
	pushk k8s-checker

.PHONY: push_public
push_public: release
	pushk --only-cluster skia-public k8s-checker
