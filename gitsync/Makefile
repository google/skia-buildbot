include ../kube/kube.mk

.PHONY: default
default: gitsync btgit

.PHONY: gitsync
gitsync:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/gitsync

.PHONY: btgit
btgit:
	go install ./go/btgit/...

.PHONY: build_release
release: gitsync build_base_cipd_release
	./build_release

.PHONY: push
push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout gitsync