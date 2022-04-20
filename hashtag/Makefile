include ../make/bazel.mk

build:
	$(BAZEL) build go/hashtag/... modules/... pages/...

release: build
	$(BAZEL) run //hashtag:push_hashtag_container

push: release
	$(BAZEL) run //kube/go/pushk -- --use-temp-checkout hashtag

run-local-instance: build
	# Run "bazel build //hashtag/..." after making any frontend changes
	# to pick up the change without restarting the golang server.
	#
	# To run locally download the service account and point to it via the
	# GOOGLE_APPLICATION_CREDENTIALS environment variable.
	cp config/config.json ../_bazel_bin/hashtag/pages/development/
	GOOGLE_APPLICATION_CREDENTIALS=${HOME}/.hashtag_creds ../_bazel_bin/hashtag/go/hashtag/hashtag_/hashtag --local --resources_dir ../_bazel_bin/hashtag/pages/development

test:
	go test ./go/...
