include ../kube/kube.mk

.PHONY: push
push: $(SKIA_PUBLIC_CONFIG_DIR)
	cd $(SKIA_PUBLIC_CONFIG_DIR) && git pull
	kubectl apply --filename=$(SKIA_PUBLIC_CONFIG_DIR)/firestore-export-everything-nightly.yaml
