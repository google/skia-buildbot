KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: all
all: frontend allgo

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

.PHONY: k8s
k8s: k8s-binaries frontend

.PHONY: k8s-binaries
k8s-binaries: skiaversion
	mkdir -p ./build
	rm -f ./build/*
	$(KGO) -o build/skiacorrectness_k8s  -a ./cmd/skiacorrectness/main.go
	$(KGO) -o build/baseline_server_k8s  -a ./cmd/baseline_server/main.go
	$(KGO) -o build/skia_diff_server_k8s -a ./cmd/skia_diff_server/main.go

include ../webtools/webtools.mk
