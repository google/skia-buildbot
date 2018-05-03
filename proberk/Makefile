default:
	go install -v ./go/proberk ./go/build_probersk_json5
	build_probersk_json5 --dest=allprobersk.json

validate: default
	proberk --config=allprobersk.json --validate

.PHONY: release
release: default
	CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./go/proberk
	./build_docker_release

.PHONY: push
push: release
	go install ../kube/go/pushk
	pushk proberk

.PHONY: run
run:
	# Runs the latest built release.
	docker run -ti -p8000:8000 proberk --config=/dev/stdin --alsologtostderr

push_config: validate
	# Need to use replace so that the configmap gets updated. Change "replace"
	# to "create" if this is the first time the configmap has been uploaded.
	kubectl create configmap proberk-conf --from-file=./allprobersk.json -o yaml --dry-run | kubectl create -f -
