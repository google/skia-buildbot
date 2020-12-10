.PHONY: android_hashlookup
android_hashlookup:
	go install -v ./android_hashlookup

.PHONY: packages
packages:
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
	echo "Be sure to have downloaded mockery 2.4.0 from"
	echo "https://github.com/vektra/mockery/releases"
	echo "and put it on your PATH"
	go generate ./...
