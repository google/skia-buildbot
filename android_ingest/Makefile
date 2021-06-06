.PHONY: build
build:
	go install -v ./go/...

.PHONY: test
test:
	go test ./go/...

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/androidingest
	./images/androidingest/release

.PHONY: push
push: release
	pushk androidingest

# Get the exact service account key, so we know that it has access to the API.
service-account.json:
	kubectl get secrets skia-android-ingest -o json | jq -r  '.data."key.json"' | base64 -d  > service-account.json

try-api: service-account.json
	go run ./go/query_buildapi

run-local-instance: build
	androidingest --logtostderr --repo_url=https://skia.googlesource.com/perf-buildid/android-master --storage_url=gs://skia-perf/android-master-ingest --work_root=/tmp