include ../make/bazel.mk
include ../kube/kube.mk

build:
	${BAZEL} build //ds/go/...

release: build
	${BAZEL} run //ds:push_datastore_backup_container

push: pushk release
	pushk --use-temp-checkout datastore-backup-k

indexes:
	gcloud datastore create-indexes ./index.yaml

cleanup-indexes:
	gcloud datastore cleanup-indexes ./index.yaml
