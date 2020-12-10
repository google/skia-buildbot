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
	echo "Be sure to have downloaded mockery 2.4.0 from"
	echo "https://github.com/vektra/mockery/releases"
	echo "and put it on your PATH"
	go generate ./...