include ../make/bazel.mk

.PHONY: build
build:
	$(BAZEL) build //android_ingest/go/...

.PHONY: test
test:
	go test ./go/...

.PHONY: release
release: build
	$(BAZEL) run //android_ingest:push_android_ingest_container

.PHONY: push
push: release
	$(BAZEL) run //kube/pushk -- --use-temp-checkout androidingest

# Get the exact service account key, so we know that it has access to the API.
service-account.json:
	kubectl get secrets skia-android-ingest -o json | jq -r  '.data."key.json"' | base64 -d  > service-account.json

try-api: service-account.json
	go run ./go/query_buildapi

run-local-instance: build
	androidingest --repo_url=https://skia.googlesource.com/perf-buildid/android-master --storage_url=gs://skia-perf/android-master-ingest --work_root=/tmp
