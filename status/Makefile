include ../make/bazel.mk

.PHONY: serve
serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run //status/modules/autoroller-status-sk:demo_page_server"

testgo:
	$(BAZEL) test --config=mayberemote //status/go/...

testjs:
	$(BAZEL) test --config=mayberemote //status/modules/...

build:
	$(BAZEL) build --config=mayberemote //status/...

build-image:
	$(BAZEL) run --config=mayberemote //status:load_status_container

.PHONY: run-local
run-local: build-image
	docker run \
		-p 8090:8090 \
		--user $$(id -u):$$(id -g) \
		-v ~/.config/gcloud:/tmp/gcloud \
		-e CLOUDSDK_CONFIG=/tmp/gcloud \
		-e GOOGLE_APPLICATION_CREDENTIALS=/tmp/gcloud/application_default_credentials.json \
		gcr.io/skia-public/status:latest \
		/usr/local/bin/status \
		--resources_dir=/usr/local/share/status \
		--host=localhost \
		--port=:8090 \
		--firestore_instance=production \
		--bigtable_project=skia-public \
		--bigtable_instance=production \
		--repo=https://skia.googlesource.com/skia.git \
		--testing
