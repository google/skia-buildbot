KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default: gitsync btgit

.PHONY: gitsync
gitsync:
	go install ./go/gitsync/...

.PHONY: btgit
btgit:
	go install ./go/btgit/...

.PHONY: build_release
 build_release: build-static-gitsync
	./build_release

.PHONY: build-static-gitsync
build-static-gitsync:
	mkdir -p ./build
	rm -f ./build/gitsync_k8s
	$(KGO) -o ./build/gitsync_k8s -a ./go/gitsync/...
