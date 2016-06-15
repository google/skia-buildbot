.PHONY: all
all: hotspare

.PHONY: hotspare
hotspare:
	go install -v ./go/hotspare

.PHONY: hotspare_release
hotspare_release: hotspare
	./build_release_hotspare "$(MESSAGE)"

.PHONY: hotspare_push
hotspare_push: hotspare
	./build_release_hotspare "`git log -n1 --format=%s`"
	pushcli hotspare skia-rpi-master-spare