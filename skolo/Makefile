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

.PHONY: backup
backup:
	go install -v ./go/rpi-backup

.PHONY: backup_release
backup_release: backup
	./build_release_backup "$(MESSAGE)"

.PHONY: backup_push
backup_push: backup
	go install -v ../push/go/pushcli
	./build_release_backup "`git log -n1 --format=%s`"
	pushcli rpi-backup skia-rpi-master

.PHONY: cloudlogger
cloudlogger:
	go install -v ./go/cloudlogger

.PHONY: cloudlogger_release
cloudlogger_release:
	GOOS=linux GOARCH=arm GOARM=7 go build -o out/cloudlogger -v ./go/cloudlogger
	go install -v ../get_service_account/go/get_service_account
	get_service_account
	./build_release_cloudlogger "$(MESSAGE)"