include ../make/bazel.mk

.PHONY: build-images
build-images:
	$(BAZEL) build --config=mayberemote //cd/go/build-images:build-images

.PHONY: release
release: build-images
	./build_release

.PHONY: docs
docs:
	$(BAZEL) run --config=mayberemote //cd/cmd/stagemanager:stagemanager -- markdown > STAGEMANAGER.md

.PHONY: build-dashboard-image
build-dashboard-image:
	$(BAZEL) run --config=mayberemote //cd:load_cd_dashboard_container

.PHONY: run-local
run-local: build-dashboard-image
	docker run \
		-p 8000:8000 \
		--user $$(id -u):$$(id -g) \
		-v ~/.config/gcloud:/tmp/gcloud \
		-e CLOUDSDK_CONFIG=/tmp/gcloud \
		-e GOOGLE_APPLICATION_CREDENTIALS=/tmp/gcloud/application_default_credentials.json \
		gcr.io/skia-public/cd-dashboard:latest \
		/usr/local/bin/dashboard \
		--resources_dir=/usr/local/share/cd-dashboard/dist \
		--port=:8000 \
		--repo=skia-public/production:https://skia.googlesource.com/buildbot.git,https://skia.googlesource.com/skia.git \
		--repo=google.com:skia-corp/internal:https://skia.googlesource.com/k8s-config.git
