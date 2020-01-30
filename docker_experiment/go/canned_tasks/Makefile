.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux go install -a ./docker-build

.PHONY: release
release: build
	./build_release

default: build
