# Go build command to build static binaries suitable for lean Alpine containers.
KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: all
all: hotspare backup trooper_tools

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
	pushcli "router-backup" "jumphost-rack-01"
	pushcli "router-backup" "jumphost-rack-02"
	pushcli "router-backup" "jumphost-rack-03"


.PHONY: trooper_tools
trooper_tools:
	go install -v ./go/powercycle-cli
	go install -v ./go/powercycle-daemon
	go install -v ./go/censustaker

.PHONY: trooper_tools_release
trooper_tools_release: trooper_tools
	./build_release_trooper_tools "`git log -n1 --format=%s`" "internal-01 rack-01 rack-02 rack-03"

.PHONY: trooper_tools_push_all
trooper_tools_push_all: trooper_tools_release
	go install -v ../push/go/pushcli
	pushcli "trooper-tools-internal-01"   "jumphost-internal-01"
	pushcli "trooper-tools-rack-01"       "jumphost-rack-01"
	pushcli "trooper-tools-rack-02"       "jumphost-rack-02"
	pushcli "trooper-tools-rack-03"       "jumphost-rack-03"

.PHONY: metadata_server
metadata_server:
	go install -v ./go/metadata_server

.PHONY: metadata_server_release
metadata_server_release: metadata_server
	./build_release_metadata_server "`git log -n1 --format=%s`"

.PHONY: metadata_server_push
metadata_server_push: metadata_server_release
	pushcli "metadata-server" "jumphost-internal-01"
	pushcli "metadata-server" "jumphost-rack-01"
	pushcli "metadata-server" "jumphost-rack-02"
	pushcli "metadata-server" "jumphost-rack-03"

.PHONY: get_oauth2_token
get_oauth2_token:
	go install -v ./go/get_oauth2_token

.PHONY: get_oauth2_token_release
get_oauth2_token_release: get_oauth2_token
	./build_release_get_oauth2_token "`git log -n1 --format=%s`"

.PHONY: get_oauth2_token_push
get_oauth2_token_push: get_oauth2_token_release
	pushcli "get-oauth2-token" "jumphost-internal-01"
	pushcli "get-oauth2-token" "jumphost-rack-01"
	pushcli "get-oauth2-token" "jumphost-rack-02"
	pushcli "get-oauth2-token" "jumphost-rack-03"

# Build all the binaries which should run in containers in the Skolo.
.PHONY: k8s-skolo-binaries
k8s-skolo-binaries:
	mkdir -p ./build
	rm -f ./build/*
	$(KGO) -o ./build/metadata-server-k8s -a ./go/metadata_server_k8s/...
	$(KGO) -o ./build/file-backup-k8s -a ./go/file-backup/...

.PHONY: k8s-push-skolo-containers
 k8s-push-skolo-containers: k8s-skolo-binaries
	./k8s_release_metadata_server
	./k8s_release_router_backup

export HASH=$(shell git rev-parse HEAD)
export DATETIME=$(shell date --utc "+%Y-%m-%dT%H:%M:%SZ")
export GIT_STATE=$(shell ../bash/gitstate.sh)
export VERSION=${DATETIME}-${USER}-${HASH}-${GIT_STATE}

# The names of this target can't change, as the form is dictated by ansible playbooks in //skolo/ansible/switchboard.
build_metadata_server_ansible_x86_64_Linux:
	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/metadata_server_ansible \
	 -ldflags="-X 'main.Version=${VERSION}' -X 'main.Key=${SERVICE_ACCOUNT_KEY}' " \
	 ./go/metadata_server_ansible


# The names of this target can't change, as the form is dictated by ansible playbooks in //skolo/ansible/switchboard.
build_powercycle_server_ansible_x86_64_Linux:
	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/powercycle_server_ansible \
	 -ldflags="-X 'main.Version=${VERSION}' " \
	 ./go/powercycle_server_ansible


# The POWERCYCLE_PASSWORD must be set in the environment for this to work correctly.
run-local-instance-powercycle_server_ansible:
	go install ./go/powercycle_server_ansible
	powercycle_server_ansible --local --powercycle_config=powercycle-rack4.json5 --config=prod.json