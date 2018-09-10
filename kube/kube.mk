pushk:
	go install ../kube/go/pushk

kube-conf-gen:
	go install -v ../kube/go/kube-conf-gen

SKIA_CORP_CONFIG_DIR="/tmp/skia-corp-config"
SKIA_PUBLIC_CONFIG_DIR="/tmp/skia-public-config"

deployment-dirs: $(SKIA_CORP_CONFIG_DIR) $(SKIA_PUBLIC_CONFIG_DIR)

$(SKIA_CORP_CONFIG_DIR):
	if [ ! -d $(SKIA_CORP_CONFIG_DIR) ]; then git clone https://skia.googlesource.com/skia-corp-config.git $(SKIA_CORP_CONFIG_DIR); fi

$(SKIA_PUBLIC_CONFIG_DIR):
	if [ ! -d $(SKIA_PUBLIC_CONFIG_DIR) ]; then git clone https://skia.googlesource.com/skia-public-config.git $(SKIA_PUBLIC_CONFIG_DIR); fi
