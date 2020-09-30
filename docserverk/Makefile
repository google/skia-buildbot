default: res/css/main.css
	go install -v ./go/docserverk

.PHONY: release
release: default
	CGO_ENABLED=0 GOOS=linux go install -a ./go/docserverk
	./build_docker_release

.PHONY: push
push: release
	go install ../kube/go/pushk
	pushk docserverk

.PHONY: run
run:
	# Runs the latest built release.
	docker run -ti -p8000:8000 docserverk --resources_dir=/usr/local/share/docserverk/ --alsologtostderr --local
