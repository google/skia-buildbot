.PHONY: all
all: hotspare backup cloudlogger

.PHONY: hotspare
hotspare:
	go install -v ./go/hotspare

.PHONY: hotspare_release
hotspare_release: hotspare
	./build_release_hotspare "$(MESSAGE)"

.PHONY: hotspare_push
hotspare_push: hotspare
	go install -v ../push/go/pushcli
	./build_release_hotspare "`git log -n1 --format=%s`"
	pushcli hotspare skia-rpi-master-spare

.PHONY: service_account
service_account:
	go install -v ../get_service_account/go/get_service_account
	get_service_account

.PHONY: backup
backup:
	go install -v ./go/file-backup

.PHONY: rpi_backup_release
rpi_backup_release: backup service_account
	./build_release_rpi_backup "$(MESSAGE)"

.PHONY: rpi_backup_push
rpi_backup_push: backup service_account
	go install -v ../push/go/pushcli
	./build_release_rpi_backup "`git log -n1 --format=%s`"
	pushcli rpi-backup skia-rpi-master

.PHONY: router_backup_release
router_backup_release: backup service_account
	./build_release_router_backup "$(MESSAGE)"

.PHONY: router_backup_push
router_backup_push: backup service_account
	go install -v ../push/go/pushcli
	./build_release_router_backup "`git log -n1 --format=%s`"
	pushcli router-backup skia-rpi-master

.PHONY: powercycle_backup_release
powercycle_backup_release: backup service_account
	./build_release_powercycle_backup "$(MESSAGE)"

.PHONY: powercycle_backup_push
powercycle_backup_push: backup service_account
	go install -v ../push/go/pushcli
	./build_release_powercycle_backup "`git log -n1 --format=%s`"
	pushcli powercycle-backup skia-jumphost

.PHONY: cloudlogger
cloudlogger:
	go install -v ./go/cloudlogger

.PHONY: cloudlogger_release
cloudlogger_release: service_account
	GOOS=linux GOARCH=arm GOARM=7 go build -o out/cloudlogger -v ./go/cloudlogger
	./build_release_cloudlogger "$(MESSAGE)"

.PHONY: android_watchdog
android_watchdog:
	@echo "  Using Android toolchain at $(NDK_TOOLCHAIN)"
	@echo "  If you haven't created a toolchain yet, download the ndk from"
	@echo "  https://developer.android.com/ndk/downloads/index.html"
	@echo "  Extract it to [NDK_DIR] (e.g. /opt/ndk/), then run"
	@echo "  [NDK_DIR]/build/tools/make_standalone_toolchain.py --arch arm --api 21 --install-dir [NDK_TOOLCHAIN]"
	@echo "  Where [NDK_TOOLCHAIN] is anywhere you like (e.g. /opt/android_toolchains)"
	CC="$(NDK_TOOLCHAIN)/bin/arm-linux-androideabi-clang" CXX="$(NDK_TOOLCHAIN)/bin/arm-linux-androideabi-clang++" GOOS=android GOARCH=arm GOARM=7 CGO_ENABLED=1 go build -o out/android_watchdog -v ./cgo/android_watchdog
