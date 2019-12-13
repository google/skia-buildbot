pushk:
	go install ../kube/go/pushk

kube-conf-gen:
	go install -v ../kube/go/kube-conf-gen

K8S_CONFIG_DIR="/tmp/k8s-config"

deployment-dirs: $(K8S_CONFIG_DIR) $(SKIA_PUBLIC_CONFIG_DIR)

$(K8S_CONFIG_DIR):
	if [ ! -d $(K8S_CONFIG_DIR) ]; then git clone https://skia.googlesource.com/k8s-config.git $(K8S_CONFIG_DIR); fi

.PHONY: build_base_cipd_release
build_base_cipd_release:
	cd ../kube && ./build_base_cipd_release
