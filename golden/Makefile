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

.PHONY: release-goldctl-gcs
release-goldctl-gcs: build-static-goldctl
	./release_goldctl

.PHONY: build-static-skiacorrectness
build-static-skiacorrectness: skiaversion
	mkdir -p ./build
	rm -f ./build/skiacorrectness_k8s
	$(KGO) -o build/skiacorrectness_k8s -a ./cmd/skiacorrectness/main.go

.PHONY: build-static-diffserver
build-static-diffserver: skiaversion
	mkdir -p ./build
	rm -f ./build/diff_server_k8s
	$(KGO) -o build/diff_server_k8s -a ./cmd/skia_diff_server/main.go

.PHONY: build-static-goldctl
build-static-goldctl:
	mkdir -p ./build
	rm -f ./build/goldctl
	$(KGO) -o build/goldctl -a ./cmd/goldctl/main.go

include ../webtools/webtools.mk
