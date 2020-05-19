.PHONY: bb_cfg_updater
bb_cfg_updater:
	go install -v ./go/bb-cfg-updater/...

.PHONY: release
release:
	CGO_ENABLED=0 GOOS=linux go install -a ./go/...
	./build_docker_release

.PHONY: push
push: release
	pushk bb-cfg-updater
