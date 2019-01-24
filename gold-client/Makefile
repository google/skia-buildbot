KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default: allgo

include ../go/skiaversion/skiaversion.mk

.PHONY: test
test:
	go test ./go/...

.PHONY: allgo
allgo: skiaversion
	go install -v ./cmd/...

.PHONY: release-goldctl-gcs
release-goldctl-gcs: build-static-goldctl
	./release_goldctl

.PHONY: build-static-goldctl
build-static-goldctl:
	mkdir -p ./build
	rm -f ./build/goldctl
	$(KGO) -o build/goldctl -a ./cmd/goldctl/...
