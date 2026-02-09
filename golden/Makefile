include ../make/bazel.mk
include ../kube/kube.mk

.PHONY: default
default:
	bazel build cmd/...

# Run a spanner emulator locally.
run_spanner_emulator:
	docker ps -q | xargs -r docker rm -vf
	docker run -d -p 5432:5432 gcr.io/cloud-spanner-pg-adapter/pgadapter-emulator:latest

run_pgadapter_infra:
	docker ps -q | xargs -r docker rm -vf
	sudo docker run -d -p 127.0.0.1:5432:5432 \
		-v $(HOME)/.config/gcloud/application_default_credentials.json:/acct_credentials.json \
		gcr.io/cloud-spanner-pg-adapter/pgadapter:latest \
		-p <Add GCP Project> -i <Add instance Id> -d <Add DB name> -c /acct_credentials.json -x


run_cockroachdb_emulators:
	../scripts/run_emulators/run_emulators start

# Requires the CockroachDB emulator to be running.
# `cockroach start-single-node --insecure`
generate_sql_schema:
	cd ./go/sql/schema; go run ../exporter/tosql --output_file sql.go --output_pkg schema
	go run ./go/sql/exportschema --out ./go/sql/expectedschema/schema.json

# Requires the Spanner emulator to be running.
generate_sql_schema_spanner:
	cd ./go/sql/schema; go run ../exporter/tosql --output_file ./spanner/sql_spanner.go --output_pkg spanner --schemaTarget spanner
	go run ./go/sql/exportschema --out ./go/sql/expectedschema/schema_spanner.json --databaseType spanner


.PHONY: frontend
frontend:
	# Makes sure all the frontend pages and tests compile.
	bazel build modules/...

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

# Run auth proxy (port 8003) in front of the local gold instance (port 8000).
.PHONY: run_auth_proxy_before_local_instance
run_auth_proxy_before_local_instance:
	$(BAZEL) run --config=mayberemote //kube/cmd/auth-proxy -- \
		--prom-port=:20003 \
		--role=editor=google.com \
		--authtype=mocked \
		--mock_user=$(USER)@google.com \
		--port=:8003 \
		--target_port=http://127.0.0.1:8000 \
		--local




run_local_frontend: run-spanner-emulator
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
		go run ./go/local --database_url="postgresql://localhost:5432/gold?sslmode=disable" --spanner
		../_bazel_bin/golden/cmd/gold_frontend/gold_frontend_/gold_frontend \
			--common_instance_config=./k8s-instances/local/local.json5 \
			--config=./k8s-instances/local/local-frontend.json5

run_local_frontend_cdb:
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local
	../_bazel_bin/golden/cmd/gold_frontend/gold_frontend_/gold_frontend \
		--common_instance_config=./k8s-instances/local/local-cdb.json5 \
		--config=./k8s-instances/local/local-frontend.json5

run_local_periodictasks: run_spanner_emulator
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local --database_url="postgresql://localhost:5432/gold?sslmode=disable" --spanner
	../_bazel_bin/golden/cmd/periodictasks/periodictasks_/periodictasks \
		--common_instance_config=./k8s-instances/local/local.json5 \
		--config=./k8s-instances/local/local-periodictasks.json5 \
		--local

run_local_periodictasks_cdb:
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local
	../_bazel_bin/golden/cmd/periodictasks/periodictasks_/periodictasks \
		--common_instance_config=./k8s-instances/local/local-cdb.json5 \
		--config=./k8s-instances/local/local-periodictasks.json5 \
		--local

run_local_gitiles_follower: run_spanner_emulator
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local --database_url="postgresql://localhost:5432/gold?sslmode=disable" --spanner
	../_bazel_bin/golden/cmd/gitilesfollower/gitilesfollower_/gitilesfollower \
		--common_instance_config=./k8s-instances/local/local.json5 \
		--config=./k8s-instances/local/local-gitilesfollower.json5

run_local_gitiles_follower_cdb:
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local
	../_bazel_bin/golden/cmd/gitilesfollower/gitilesfollower_/gitilesfollower \
		--common_instance_config=./k8s-instances/local/local-cdb.json5 \
		--config=./k8s-instances/local/local-gitilesfollower.json5

run_local_gold_ingestion: run_spanner_emulator
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local --database_url="postgresql://localhost:5432/gold?sslmode=disable" --spanner
	../_bazel_bin/golden/cmd/gold_ingestion/gold_ingestion_/gold_ingestion \
		--common_instance_config=./k8s-instances/local/local.json5 \
		--config=./k8s-instances/local/local-ingestion.json5

run_local_gold_ingestion_cdb:
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local
	../_bazel_bin/golden/cmd/gold_ingestion/gold_ingestion_/gold_ingestion \
		--common_instance_config=./k8s-instances/local/local-cdb.json5 \
		--config=./k8s-instances/local/local-ingestion.json5

run_local_diffcalculator: run-spanner-emulator
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local --database_url="postgresql://localhost:5432/gold?sslmode=disable" --spanner
	GOLD_DIFFCALC_IMAGESOURCE_TYPE=LOCAL ../_bazel_bin/golden/cmd/diffcalculator/diffcalculator_/diffcalculator \
		--common_instance_config=./k8s-instances/local/local.json5 \
		--config=./k8s-instances/local/local-diffcalculator.json5

run_local_diffcalculator_cdb:
	$(BAZEL) build --config=mayberemote -c dbg //golden/...
	go run ./go/local
	GOLD_DIFFCALC_IMAGESOURCE_TYPE=LOCAL ../_bazel_bin/golden/cmd/diffcalculator/diffcalculator_/diffcalculator \
		--common_instance_config=./k8s-instances/local/local-cdb.json5 \
		--config=./k8s-instances/local/local-diffcalculator.json5

run_pgadapter:
	docker ps -q | xargs -r docker rm -vf
	sudo docker run -d -p 127.0.0.1:5432:5432 \
  		-v $(HOME)/.config/gcloud/application_default_credentials.json:/acct_credentials.json \
		gcr.io/cloud-spanner-pg-adapter/pgadapter:latest \
		-p skia-infra-corp -i tfgen-spanid-20250415224933743 -d ashwinpv_test -c /acct_credentials.json -x

run_local_migrator: run_spanner_emulator
	$(BAZEL) build --config=mayberemote -c dbg //golden/cmd/db_migrator/...
	../_bazel_bin/golden/cmd/db_migrator/db_migrator_/db_migrator \
		--cdb_url="postgresql://root@localhost:26257/gold?sslmode=disable" \
		--spanner_url="postgresql://localhost:5432/gold?sslmode=disable" \
		--continuous

run_migrator_infra: run_pgadapter_infra
	$(BAZEL) build --config=mayberemote -c dbg //golden/cmd/db_migrator/...
	../_bazel_bin/golden/cmd/db_migrator/db_migrator_/db_migrator \
		--cdb_url="postgresql://root@localhost:26235/skiainfra?sslmode=disable" \
		--spanner_url="postgresql://localhost:5432/skia-infra?sslmode=disable" \
		--continuous