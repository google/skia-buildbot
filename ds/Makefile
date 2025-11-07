include ../make/bazel.mk

build:
	${BAZEL} build //ds/go/...

release: build
	${BAZEL} run //ds:push_datastore_backup_container

indexes:
	gcloud datastore create-indexes ./index.yaml

cleanup-indexes:
	gcloud datastore cleanup-indexes ./index.yaml
