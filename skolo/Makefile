include ../go/skiaversion/skiaversion.mk

.PHONY: all
all: hotspare backup cloudlogger trooper_tools

.PHONY: hotspare
hotspare:
	go install -v ./go/hotspare

.PHONY: hotspare_release
hotspare_release: hotspare
	./build_release_hotspare "`git log -n1 --format=%s`"

.PHONY: hotspare_push
hotspare_push: hotspare
	go install -v ../push/go/pushcli
	./build_release_hotspare "`git log -n1 --format=%s`"
	pushcli hotspare skia-rpi-master-spare

.PHONY: backup
backup:
	go install -v ./go/file-backup

.PHONY: rpi_backup_release
rpi_backup_release: backup
	./build_release_rpi_backup "`git log -n1 --format=%s`"

.PHONY: rpi_backup_push
rpi_backup_push: backup
	go install -v ../push/go/pushcli
	./build_release_rpi_backup "`git log -n1 --format=%s`"
	pushcli rpi-backup skia-rpi-master

.PHONY: router_backup_release
router_backup_release: backup
	./build_release_router_backup "`git log -n1 --format=%s`"

.PHONY: router_backup_push_all
router_backup_push_all: backup
	go install -v ../push/go/pushcli
	./build_release_router_backup "`git log -n1 --format=%s`"
	pushcli "router-backup" "jumphost-internal-01"
	pushcli "router-backup" "jumphost-rpi-01"
	pushcli "router-backup" "jumphost-linux-01"
	pushcli "router-backup" "jumphost-win-02"
	pushcli "router-backup" "jumphost-win-03"


.PHONY: cloudlogger
cloudlogger:
	go install -v ./go/cloudlogger

.PHONY: cloudlogger_release
cloudlogger_release:
	GOOS=linux GOARCH=arm GOARM=7 go build -o out/cloudlogger -v ./go/cloudlogger
	./build_release_cloudlogger "`git log -n1 --format=%s`"

.PHONY: trooper_tools
trooper_tools:
	go install -v ./go/powercycle-cli
	go install -v ./go/powercycle-daemon
	go install -v ./go/censustaker

.PHONY: trooper_tools_release
trooper_tools_release: trooper_tools
	./build_release_trooper_tools "`git log -n1 --format=%s`" "internal-01 linux-01 rpi-01 win-02 win-03"

.PHONY: trooper_tools_push_all
trooper_tools_push_all: trooper_tools_release
	go install -v ../push/go/pushcli
	pushcli "trooper-tools-internal-01"   "jumphost-internal-01"
	pushcli "trooper-tools-rpi-01"        "jumphost-rpi-01"
	pushcli "trooper-tools-linux-01"      "jumphost-linux-01"
	pushcli "trooper-tools-win-02"        "jumphost-win-02"
	pushcli "trooper-tools-win-03"        "jumphost-win-03"

.PHONY: android_watchdog
android_watchdog:
	@echo "  Using Android toolchain at $(NDK_TOOLCHAIN)"
	@echo "  If you haven't created a toolchain yet, download the ndk from"
	@echo "  https://developer.android.com/ndk/downloads/index.html"
	@echo "  Extract it to [NDK_DIR] (e.g. /opt/ndk/), then run"
	@echo "  [NDK_DIR]/build/tools/make_standalone_toolchain.py --arch arm --api 21 --install-dir [NDK_TOOLCHAIN]"
	@echo "  Where [NDK_TOOLCHAIN] is anywhere you like (e.g. /opt/android_toolchains)"
	CC="$(NDK_TOOLCHAIN)/bin/arm-linux-androideabi-clang" CXX="$(NDK_TOOLCHAIN)/bin/arm-linux-androideabi-clang++" GOOS=android GOARCH=arm GOARM=7 CGO_ENABLED=1 go build -o out/android_watchdog -v ./cgo/android_watchdog

.PHONY: metadata_server
metadata_server:
	go install -v ./go/metadata_server

.PHONY: metadata_server_release
metadata_server_release: metadata_server
	./build_release_metadata_server "`git log -n1 --format=%s`"

.PHONY: metadata_server_push
metadata_server_push: metadata_server_release
	pushcli "metadata-server" "jumphost-internal-01"
	pushcli "metadata-server" "jumphost-linux-01"
	pushcli "metadata-server" "jumphost-rpi-01"
	pushcli "metadata-server" "jumphost-win-02"
	pushcli "metadata-server" "jumphost-win-03"

.PHONY: get_oauth2_token
get_oauth2_token:
	go install -v ./go/get_oauth2_token

.PHONY: get_oauth2_token_release
get_oauth2_token_release: get_oauth2_token
	./build_release_get_oauth2_token "`git log -n1 --format=%s`"

.PHONY: get_oauth2_token_push
get_oauth2_token_push: get_oauth2_token_release
	pushcli "get-oauth2-token" "jumphost-internal-01"
	pushcli "get-oauth2-token" "jumphost-linux-01"
	pushcli "get-oauth2-token" "jumphost-rpi-01"
	pushcli "get-oauth2-token" "jumphost-win-02"
	pushcli "get-oauth2-token" "jumphost-win-03"
