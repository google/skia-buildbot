.PHONY: android_hashlookup
android_hashlookup:
	go install -v ./android_hashlookup

.PHONY: packages
packages:
	go install -v ./...

.PHONY: testgo
testgo:
	go test ./... -v -test.short

.PHONY: all
all: packages android_hashlookup
