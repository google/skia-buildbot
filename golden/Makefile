KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default: frontend allgo

include ../go/skiaversion/skiaversion.mk

.PHONY: packages
packages:
	go build -v ./go/...

# This is the quick version triggered by the presubmit script.
.PHONY: testgo
testgo:
	go test ./go/... -short

.PHONY: bench
bench:
	go test ./go/... -bench=. -run=NONE

.PHONY: test
test:
	go test ./go/...

node_modules: package.json
	npm install

.PHONY: frontend
frontend:
	cd frontend && $(MAKE) web

.PHONY: allgo
allgo: skiaversion
	go install -v ./cmd/...

.PHONY: k8s-release-skiacorrectness
k8s-release-skiacorrectness: build-static-skiacorrectness frontend
	./k8s_release_skiacorrectness

.PHONY: k8s-release-diff-server
k8s-release-diff-server: build-static-diffserver
	./k8s_release_diff_server

.PHONY: k8s-release-baseline-server
k8s-release-baseline-server: build-static-baseline-server
	./k8s_release_baseline_server

.PHONY: build-static-skiacorrectness
build-static-skiacorrectness: skiaversion
	mkdir -p ./build
	rm -f ./build/skiacorrectness_k8s
	$(KGO) -o build/skiacorrectness_k8s -a ./cmd/skiacorrectness/main.go

.PHONY: build-static-baseline-server
build-static-baseline-server: skiaversion
	mkdir -p ./build
	rm -f ./build/baseline_server_k8s
	$(KGO) -o build/baseline_server_k8s -a ./cmd/baseline_server/main.go

.PHONY: build-static-diffserver
build-static-diffserver: skiaversion
	mkdir -p ./build
	rm -f ./build/diff_server_k8s
	$(KGO) -o build/diff_server_k8s -a ./cmd/skia_diff_server/main.go

.PHONY: release
release: allgo frontend
	./build_release "release" "all"

.PHONY: release_diffserver
release_diffserver: allgo
	./build_release_diffserver "release" "all"


.PHONY: mocks
mocks:
	go get github.com/vektra/mockery

	mockery -name GCSClient -dir go/storage/ -output ./go/mocks/
	mockery -name TileInfo -dir go/baseline/ -output ./go/mocks/
	mockery -name ExpectationsStore -dir go/expstorage/ -output ./go/mocks/
	mockery -name ComplexTile -dir go/types/ -output ./go/mocks/
	mockery -name DigestCounter -dir go/digest_counter/ -output ./go/mocks/
	mockery -name TileSource -dir go/storage/ -output ./go/mocks/

include ../webtools/webtools.mk
