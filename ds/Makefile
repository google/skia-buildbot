include ../make/bazel.mk

build:
	${BAZEL} build //ds/go/...

release: build
	${BAZEL} run //ds:push_datastore_backup_container

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout datastore-backup-k

indexes:
	gcloud datastore create-indexes ./index.yaml

cleanup-indexes:
	gcloud datastore cleanup-indexes ./index.yaml
