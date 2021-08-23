KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default:
	bazel build cmd/...

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

# Build the gold-frontend container with Bazel and push it to GCR.
.PHONY: push_frontend_container
push_frontend_container:
	bazel run //golden:push_gold_frontend_container

# Build the gold-baseline-server container with Bazel and push it to GCR.
.PHONY: push_baseline_server_container
push_baseline_server_container:
	bazel run //golden:push_baseline_server_container

# Build the gold-diffcalculator container with Bazel and push it to GCR.
.PHONY: push_diffcalculator_container
push_diffcalculator_container:
	bazel run //golden:push_diffcalculator_container

# Build the gold-gitilesfollower container with Bazel and push it to GCR.
.PHONY: push_gitilesfollower_container
push_gitilesfollower_container:
	bazel run //golden:push_gitilesfollower_container

# Build the gold-ingestion container with Bazel and push it to GCR.
.PHONY: push_ingestion_container
push_ingestion_container:
	bazel run //golden:push_gold_ingestion_container

# Build the gold-periodictasks container with Bazel and push it to GCR.
.PHONY: push_periodictasks_container
push_periodictasks_container:
	bazel run //golden:push_periodictasks_container

.PHONY: mocks
mocks:
	echo "Be sure to have downloaded mockery 2.4.0 from"
	echo "https://github.com/vektra/mockery/releases"
	echo "and put it on your PATH"
	go generate ./...
