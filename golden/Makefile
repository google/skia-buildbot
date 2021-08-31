include ../make/bazel.mk

.PHONY: default
default:
	bazel build cmd/...

.PHONY: goldpushk
goldpushk:
	go install ./cmd/goldpushk/...

# Build the gold-frontend container with Bazel and push it to GCR.
.PHONY: push_frontend_container
push_frontend_container:
	$(BAZEL) run //golden:push_gold_frontend_container

# Build the gold-baseline-server container with Bazel and push it to GCR.
.PHONY: push_baseline_server_container
push_baseline_server_container:
	$(BAZEL) run //golden:push_baseline_server_container

# Build the gold-diffcalculator container with Bazel and push it to GCR.
.PHONY: push_diffcalculator_container
push_diffcalculator_container:
	$(BAZEL) run //golden:push_diffcalculator_container

# Build the gold-gitilesfollower container with Bazel and push it to GCR.
.PHONY: push_gitilesfollower_container
push_gitilesfollower_container:
	$(BAZEL) run //golden:push_gitilesfollower_container

# Build the gold-ingestion container with Bazel and push it to GCR.
.PHONY: push_ingestion_container
push_ingestion_container:
	$(BAZEL) run //golden:push_gold_ingestion_container

# Build the gold-periodictasks container with Bazel and push it to GCR.
.PHONY: push_periodictasks_container
push_periodictasks_container:
	$(BAZEL) run //golden:push_periodictasks_container

.PHONY: mocks
mocks:
	echo "Be sure to have downloaded mockery 2.4.0 from"
	echo "https://github.com/vektra/mockery/releases"
	echo "and put it on your PATH"
	go generate ./...
