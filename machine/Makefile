include ../make/bazel.mk

build:
	$(BAZEL) build ...

generate:
	go generate ./go/...

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/machine-server-sk:demo_page_server"

testjs:
	$(BAZEL) test modules/...

release:
	$(BAZEL) build --config=remote //machine/...
	./images/machineserver/release

push: release
	pushk machineserver

push_bazel:
	$(BAZEL) run //machine:push_machineserver
	pushk machineserver

# Make our own firestore emulator image.
# https://issuetracker.google.com/issues/153087269
release_firestore_emulator:
	./images/firestore-emulator/release

start_firestore_emulator:
	 docker run -ti -p 8011:8011 gcr.io/skia-public/firestore-emulator:2020-04-06T16_43_02Z-jcgregorio-bb5fa1d-dirty gcloud beta emulators firestore start --project testtest --host-port 0.0.0.0:8011

start_pubsub_emulator:
	docker run -ti -p 8010:8010 google/cloud-sdk:latest gcloud beta emulators pubsub start \
		  --project test-project --host-port 0.0.0.0:8010

build-remote:
	$(BAZEL) build //machine/go/machineserver:machineserver --config=remote --remote_instance_name=projects/skia-public/instances/default_instance

run-local-instance: build
	../_bazel_bin/machine/go/machineserver/machineserver_/machineserver --local --resources_dir ../_bazel_bin/machine/pages/development

export HASH:=$(shell git rev-parse HEAD)
export DATETIME:=$(shell date -u "+%Y-%m-%dT%H:%M:%SZ")
export GIT_STATE:=$(shell ../bash/gitstate.sh)
export VERSION:=${DATETIME}-${USER}-${HASH}-${GIT_STATE}

build_test_machine_monitor_for_all_archs:
	rm -rf ./build

	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=arm64 \
	go build \
	  -o ./build/Linux/aarch64/test_machine_monitor \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	  ./go/test_machine_monitor

	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/test_machine_monitor \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	 ./go/test_machine_monitor

	CGO_ENABLED=0 \
	GOOS=darwin \
	GOARCH=amd64 \
	go build \
	 -o ./build/Darwin/x86_64/test_machine_monitor \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	 ./go/test_machine_monitor

	CGO_ENABLED=0 \
	GOOS=darwin \
	GOARCH=arm64 \
	go build \
	 -o ./build/Darwin/arm64/test_machine_monitor \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	 ./go/test_machine_monitor

	CGO_ENABLED=0 \
	GOOS=windows \
	GOARCH=amd64 \
	go build \
	 -o ./build/Win32NT/64-bit/test_machine_monitor \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	 ./go/test_machine_monitor

build_and_upload_test_machine_monitor: build_test_machine_monitor_for_all_archs
	cipd create -pkg-def=test_machine_monitor_cipd.yml --tag version:${VERSION}

	#
	# The instance can be retrieved using the version tag:
	#
	#   cipd install --root=/tmp/sometempdir skia/internal/test_machine_monitor version:${VERSION}

# After running this, you may want to deploy install the tmm executable. To do so, see
# //skolo/ansible/switchboard/roles/install_test_machine_monitor/README.md
release_test_machine_monitor: build_and_upload_test_machine_monitor
	../bash/ansible-release.sh test_machine_monitor ${VERSION}

.PHONY: build generate serve testjs release push push_bazel \
    release_firestore_emulator start_firestore_emulator start_pubsub_emulator \
    build-remote run-local-instance build \
    build_test_machine_monitor_for_all_archs \
    build_and_upload_test_machine_monitor \
    build_test_machine_monitor_for_all_archs release_test_machine_monitor \
    build_and_upload_test_machine_monitor
