include ../make/bazel.mk

.PHONY: build-image
build-image:
	$(BAZEL) run --config=mayberemote //autogardener:load_autogardener_container

.PHONY: run-local
run-local: build-image
	docker run \
		--user $$(id -u):$$(id -g) \
		-v ~/.config/gcloud:/tmp/gcloud \
		-e CLOUDSDK_CONFIG=/tmp/gcloud \
		-e GOOGLE_APPLICATION_CREDENTIALS=/tmp/gcloud/application_default_credentials.json \
		-e GEMINI_API_KEY=$(GEMINI_API_KEY) \
		--net=host \
		gcr.io/skia-public/autogardener:latest \
		autogardener \
		--firestore-instance=production \
		--bigtable-project=skia-public \
		--bigtable-instance=production \
		--repo=https://skia.googlesource.com/skia.git \
		--local
