include ../make/bazel.mk

.PHONY: backup-to-gcs
backup-to-gcs:
	$(BAZEL) run //promk:push_backup_to_gcs_container
	$(BAZEL) run //kube/go/pushk -- backup-to-gcs

include ../make/clusters.mk
