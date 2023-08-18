include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: default
default:
	bazel build cmd/...

.PHONY: frontend
frontend:
	# Makes sure all the frontend pages and tests compile.
	bazel build modules/...

.PHONY: goldpushk
goldpushk: kube-conf-gen
	go install ./cmd/goldpushk/...

# Build the gold-frontend container with Bazel and push it to GCR.
.PHONY: release_frontend_container
release_frontend_container:
	$(BAZEL) run --config=mayberemote //golden:push_gold_frontend_container

# Build the gold-baseline-server container with Bazel and push it to GCR.
.PHONY: release_baseline_server_container
release_baseline_server_container:
	$(BAZEL) run --config=mayberemote //golden:push_baseline_server_container

# Build the gold-diffcalculator container with Bazel and push it to GCR.
.PHONY: release_diffcalculator_container
release_diffcalculator_container:
	$(BAZEL) run --config=mayberemote //golden:push_diffcalculator_container

# Build the gold-gitilesfollower container with Bazel and push it to GCR.
.PHONY: release_gitilesfollower_container
release_gitilesfollower_container:
	$(BAZEL) run --config=mayberemote //golden:push_gitilesfollower_container

# Build the gold-ingestion container with Bazel and push it to GCR.
.PHONY: release_ingestion_container
release_ingestion_container:
	$(BAZEL) run --config=mayberemote //golden:push_gold_ingestion_container

# Build the gold-periodictasks container with Bazel and push it to GCR.
.PHONY: release_periodictasks_container
release_periodictasks_container:
	$(BAZEL) run --config=mayberemote //golden:push_periodictasks_container

.PHONY: mocks
mocks:
	go generate ./...
