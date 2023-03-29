KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default: sk

.PHONY: test
test:
	go test ./go/... -v

.PHONY: sk
sk:
	go install -v ./go/sk

.PHONY: mocks
mocks:
	bazel run //:go -- generate ./...
