.PHONY: build
build:
	go install -v ./go/debugger

.PHONY: image
image:
	CGO_ENABLED=0 GOOS=linux go install ./go/debugger
	SKIP_UPLOAD=1 ./build_docker_release

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/debugger
	docker pull gcr.io/skia-public/skia-release:prod
	./build_docker_release

.PHONY: release_ci
release_ci:
	# Don't docker pull here, as we are in the CI pipeline and it already has
	# done the pull.
	CGO_ENABLED=0 GOOS=linux go install -a ./go/debugger
	./build_docker_release

.PHONY: push
push: release
	go install ../kube/go/pushk
	pushk debugger

.PHONY: testgo
testgo:
	go test -v ./...

.PHONY: run
run:
	docker run -p8000:8000 -ti debugger

.PHONY: run_with_local_assets
run_with_local_assets:
	docker run -p8000:8000 -ti debugger --source=http://localhost:9000 --logtostderr --resources_dir=/usr/local/share/debugger

.PHONY: tag_skiaserve
tag_skiaserve:
	./tag-latest-skiaserve.sh
