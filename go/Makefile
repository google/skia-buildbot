.PHONY: android_hashlookup
android_hashlookup:
	go install -v ./android_hashlookup

.PHONY: packages
packages:
	go install -v ./...

.PHONY: testgo
testgo: pdfium
	go test ./... -v -test.short

.PHONY: all
all: packages android_hashlookup pdfium

.PHONY: pdfium
pdfium: ${GOPATH}/bin/pdfium_test

${GOPATH}/bin/pdfium_test: pdf/build_pdfium.sh
	sh pdf/build_pdfium.sh "${GOPATH}/bin" "pdf/build"
