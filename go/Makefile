.PHONY: android_hashlookup
android_hashlookup:
	go install -v ./android_hashlookup

.PHONY: packages
packages:
	go install go.chromium.org/luci/client/cmd/isolate
	go install -v ./...

.PHONY: testgo
testgo: protobuf
	go test ./... -v -test.short

.PHONY: test
test: protobuf
	go test ./... -v

.PHONY: all
all: packages android_hashlookup

.PHONY: protobuf
protobuf:
	go generate ./...

.PHONY: mocks
mocks:
	go get github.com/vektra/mockery/...

	mockery -name EventBus -dir eventbus/ -output ./eventbus/mocks/
	mockery -name GitStore -dir gitstore/ -output ./gitstore/mocks/
	mockery -name GerritInterface -dir gerrit/ -output ./gerrit/mocks/
