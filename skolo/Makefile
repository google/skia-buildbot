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

export HASH:=$(shell git rev-parse HEAD)
export DATETIME:=$(shell date -u "+%Y-%m-%dT%H:%M:%SZ")
export GIT_STATE:=$(shell ../bash/gitstate.sh)
export VERSION:=${DATETIME}-${USER}-${HASH}-${GIT_STATE}

# This build must be launched from an Ansible playbook which will extract the
# service account keys and base64 encode them into the SERVICE_ACCOUNT_KEY and
# INTERNAL_SERVICE_ACCOUNT_KEY environment variables.
build_metadata_server_ansible:
	rm -rf ./build

	test -n "${SERVICE_ACCOUNT_KEY}" || (echo "SERVICE_ACCOUNT_KEY must be set by Ansible playbook."; exit 1)
	test -n "${INTERNAL_SERVICE_ACCOUNT_KEY}" || (echo "INTERNAL_SERVICE_ACCOUNT_KEY must be set by Ansible playbook."; exit 1)

	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/skolo-service-accounts/metadata_server_ansible \
	 -ldflags="-X 'main.Version=${VERSION}' -X 'main.Key=${SERVICE_ACCOUNT_KEY}' " \
	 ./go/metadata_server_ansible

	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/skolo-internal-service-account/metadata_server_ansible \
	 -ldflags="-X 'main.Version=${VERSION}' -X 'main.Key=${INTERNAL_SERVICE_ACCOUNT_KEY}' " \
	 ./go/metadata_server_ansible

build_and_upload_metadata_server_ansible: build_metadata_server_ansible
	cipd create -pkg-def=metadata_server_ansible_cipd.yml --tag version:${VERSION}

release_metadata_server_ansible: build_and_upload_metadata_server_ansible
	../bash/ansible-release.sh metadata_server_ansible ${VERSION}

build_powercycle_server_ansible:
	rm -rf ./build
	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/powercycle_server_ansible \
	 -ldflags="-X 'main.Version=${VERSION}' " \
	 ./go/powercycle_server_ansible

build_and_upload_powercycle_server_ansible: build_powercycle_server_ansible
	cipd create -pkg-def=powercycle_server_ansible_cipd.yml --tag version:${VERSION}

release_powercycle_server_ansible: build_and_upload_powercycle_server_ansible
	../bash/ansible-release.sh powercycle_server_ansible ${VERSION}

build_router_backup_ansible:
	rm -rf ./build
	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/router_backup_ansible \
	 -ldflags="-X 'main.Version=${VERSION}' " \
	 ./go/router_backup_ansible

build_and_upload_router_backup_ansible: build_router_backup_ansible
	cipd create -pkg-def=router_backup_ansible_cipd.yml --tag version:${VERSION}

release_router_backup_ansible: build_and_upload_router_backup_ansible
	../bash/ansible-release.sh router_backup_ansible ${VERSION}

# The POWERCYCLE_PASSWORD must be set in the environment for this to work correctly.
run-local-instance-powercycle_server_ansible:
	go install ./go/powercycle_server_ansible
	powercycle_server_ansible --local --powercycle_config=powercycle-rack4.json5 --config=prod.json

run-local-instance-router_backup_ansible:
	go install ./go/router_backup_ansible
	router_backup_ansible --local \
	  --gce_bucket skia-backups \
      --gce_folder router_config_test_backups \
      --remote_file_path "admin@router-rack1:/config/config.boot"

# The PROMETHEUS_VERSION env variable must also be set.
download_prometheus_x86_64_Linux:
ifeq ("$(wildcard ./build/Linux/x86_64/prometheus-$(PROMETHEUS_VERSION).linux-amd64/prometheus)", "")
	mkdir -p ./build/Linux/x86_64
	wget https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-amd64.tar.gz -O - | tar -xz --directory=./build/Linux/x86_64
endif
	-echo Download complete.
