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
test: pdfium protobuf
	go test ./... -v

.PHONY: all
all: packages android_hashlookup

.PHONY: pdfium
pdfium:
	../pdfium/install_pdfium.sh

.PHONY: protobuf
protobuf:
	go generate ./...
