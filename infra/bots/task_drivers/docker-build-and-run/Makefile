.PHONY: build
build:
	CGO_ENABLED=0 GOOS=linux go install -a ./docker-build-and-run.go

.PHONY: release
release: build
	./build_release
