KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default: allgo

.PHONY: test
test:
	go test ./go/... -v

.PHONY: allgo
allgo:
	go install -v ./cmd/...

.PHONY: release-goldctl-gcs
release-goldctl-gcs: build-static-goldctl
	./release_goldctl

.PHONY: build-static-goldctl
build-static-goldctl:
	mkdir -p ./build
	rm -f ./build/goldctl
	$(KGO) -o build/goldctl -a ./cmd/goldctl/...

.PHONY: mocks
mocks:
	go get github.com/vektra/mockery/...

	go generate ./...