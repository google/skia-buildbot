default:
	go install -v ./go/proberk ./go/build_probersk_json5
	build_probersk_json5 --dest=allprobersk.json

.PHONY: validate
validate: default
	proberk --config=allprobersk.json --validate

.PHONY: release
release: default
	CGO_ENABLED=0 GOOS=linux go install -a ./go/proberk
	./build_docker_release

.PHONY: push
push: release skia-public
	go install ../kube/go/pushk
	pushk proberk

include ../make/clusters.mk
