.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux go build -a ./docker_build_and_run.go

.PHONY: release
release: build
	./build_release
