include ../make/bazel.mk

.PHONY: backup-to-gcs
backup-to-gcs:
	$(BAZEL) run --config=mayberemote //promk:push_backup_to_gcs_container
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- backup-to-gcs

include ../make/clusters.mk
