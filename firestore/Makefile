include ../make/clusters.mk

.PHONY: push
push: $(SKIA_PUBLIC_CONFIG_DIR) skia-public
	cd $(SKIA_PUBLIC_CONFIG_DIR) && git pull
	kubectl apply --filename=$(SKIA_PUBLIC_CONFIG_DIR)/firestore-export-everything-nightly.yaml
