KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default: gitsync btgit

.PHONY: gitsync
gitsync:
	go install ./cmd/gitsync/...

.PHONY: btgit
btgit:
	go install ./cmd/btgit/...

.PHONY: k8s-release-gitsync
 k8s-release-gitsync: build-static-gitsync
	./k8s-release-gitsync

.PHONY: build-static-gitsync
build-static-gitsync:
	mkdir -p ./build
	rm -f ./build/gitsync_k8s
	$(KGO) -o ./build/gitsync_k8s -a ./cmd/gitsync/...
