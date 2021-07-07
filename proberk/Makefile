default:
	go install -v ./go/proberk ./go/build_probersk_json
	build_probersk_json --dest=allprobersk.json

.PHONY: validate
validate: default
	proberk --config=allprobersk.json --validate

.PHONY: release
release: default
	CGO_ENABLED=0 GOOS=linux go install -a ./go/proberk
	./build_docker_release

.PHONY: update-expectations
update-expectations:
	curl https://skia.googlesource.com?format=JSON > expectations/gob.json

.PHONY: push
push: release
	go install ../kube/go/pushk
	pushk proberk

run-local-instance:
	proberk --local --logtostderr --run_every=1s --config=./allprobersk.json