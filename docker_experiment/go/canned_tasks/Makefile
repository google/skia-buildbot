.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux go install -a ./docker-build
	CGO_ENABLED=0 GOOS=linux go install -a ./docker-run

.PHONY: release
release: build
	./build_release

.PHONY: default
default: build
