KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default: frontend allgo

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
test: go-test js-test puppeteer-test
	true

.PHONY: go-test
go-test:
	go test ./go/...

.PHONY: js-test
js-test:
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: js-test-debug
js-test-debug:
	npx karma start --no-single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: puppeteer-test
puppeteer-test:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts
	echo "results are found in ../puppeteer-tests/output"

.PHONY: puppeteer-test-debug
puppeteer-test-debug:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts --inspect-brk
	echo "results are found in ../puppeteer-tests/output"

node_modules: package.json
	npm ci

.PHONY: frontend
frontend:
	npx webpack-cli --mode=production

.PHONY: frontend-debug
frontend-debug:
	npx webpack-cli --mode=development

.PHONY: serve
serve:
	npx webpack-dev-server --watch-poll --mode=development

.PHONY: allgo
allgo:
	go install -v ./cmd/...

.PHONY: k8s-release-frontend
k8s-release-frontend: build-static-frontend frontend
	./k8s_release_frontend

.PHONY: k8s-release-baseline-server
k8s-release-baseline-server: build-static-baseline-server
	./k8s_release_baseline_server

.PHONY: k8s-release-diffcalculator
k8s-release-diffcalculator: build-static-diffcalculator
	./k8s_release_diffcalculator

.PHONY: k8s-release-gitilesfollower
k8s-release-gitilesfollower: build-static-gitilesfollower
	./k8s_release_gitilesfollower

.PHONY: k8s-release-ingestion
k8s-release-ingestion: build-static-ingestion
	./k8s_release_ingestion

.PHONY: k8s-release-periodictasks
k8s-release-periodictasks: build-static-periodictasks
	./k8s_release_periodictasks

.PHONY: k8s-release-goldpushk-healthy-server
k8s-release-goldpushk-healthy-server: build-static-goldpushk-healthy-server
	./k8s_release_goldpushk_healthy_server

.PHONY: k8s-release-goldpushk-crashing-server
k8s-release-goldpushk-crashing-server: build-static-goldpushk-crashing-server
	./k8s_release_goldpushk_crashing_server

.PHONY: build-static-frontend
build-static-frontend: frontend
	mkdir -p ./build
	rm -f ./build/frontend_k8s
	$(KGO) -o build/frontend_k8s -a ./cmd/gold_frontend/...

.PHONY: build-static-baseline-server
build-static-baseline-server:
	mkdir -p ./build
	rm -f ./build/baseline_server_k8s
	$(KGO) -o build/baseline_server_k8s -a ./cmd/baseline_server/...

.PHONY: build-static-diffcalculator
build-static-diffcalculator:
	mkdir -p ./build
	rm -f ./build/diffcalculator_k8s
	$(KGO) -o build/diffcalculator_k8s -a ./cmd/diffcalculator/...

.PHONY: build-static-gitilesfollower
build-static-gitilesfollower:
	mkdir -p ./build
	rm -f ./build/gitilesfollower_k8s
	$(KGO) -o build/gitilesfollower_k8s -a ./cmd/gitilesfollower/gitilesfollower.go

.PHONY: build-static-ingestion
build-static-ingestion:
	mkdir -p ./build
	rm -f ./build/gold_ingestion_k8s
	$(KGO) -o build/gold_ingestion_k8s -a ./cmd/gold_ingestion/...

.PHONY: build-static-periodictasks
build-static-periodictasks:
	mkdir -p ./build
	rm -f ./build/periodictasks_k8s
	$(KGO) -o build/periodictasks_k8s -a ./cmd/periodictasks/...

.PHONY: build-static-goldpushk-healthy-server
build-static-goldpushk-healthy-server:
	mkdir -p ./build
	rm -f ./build/goldpushk_healthy_server_k8s
	$(KGO) -o build/goldpushk_healthy_server_k8s -a ./cmd/goldpushk/testing/healthy_server/...

.PHONY: build-static-goldpushk-crashing-server
build-static-goldpushk-crashing-server:
	mkdir -p ./build
	rm -f ./build/goldpushk_crashing_server_k8s
	$(KGO) -o build/goldpushk_crashing_server_k8s -a ./cmd/goldpushk/testing/crashing_server/...

.PHONY: mocks
mocks:
	echo "Be sure to have downloaded mockery 2.4.0 from"
	echo "https://github.com/vektra/mockery/releases"
	echo "and put it on your PATH"
	go generate ./...

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci:
	npm ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run
