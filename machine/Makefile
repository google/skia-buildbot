include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote //machine/...

test:
	$(BAZEL) test --config=mayberemote //machine/...

# Requires the CockroachDB emulator to be running.
generate_sql_schema:
	go run ./go/machine/store/cdb/exportschema --out ./go/machine/store/cdb/expectedschema/schema.json

generate: generate_sql_schema
	go generate ./go/...

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/machine-server-sk:demo_page_server"

testjs:
	$(BAZEL) test --config=mayberemote modules/...

release:
	$(BAZEL) run --config=mayberemote //machine:push_machineserver

# Make our own firestore emulator image.
# https://issuetracker.google.com/issues/153087269
release_firestore_emulator:
	./images/firestore-emulator/release

start_firestore_emulator:
	 docker run -ti -p 8011:8011 gcr.io/skia-public/firestore-emulator:2020-04-06T16_43_02Z-jcgregorio-bb5fa1d-dirty gcloud beta emulators firestore start --project testtest --host-port 0.0.0.0:8011

start_pubsub_emulator:
	docker run -ti -p 8010:8010 google/cloud-sdk:latest gcloud beta emulators pubsub start \
		  --project test-project --host-port 0.0.0.0:8010

# Presumes you've already run:
#
#    make build
#
#    kubectl port-forward service/machineserver-cockroachdb 25001:26257
#
# Run "bazel build //perf/..." after making any frontend changes to pick up the
# change without restarting the golang server.
run-local-instance: build
	../_bazel_bin/machine/go/machineserver/machineserver_/machineserver --local --resources_dir ../_bazel_bin/machine/pages/development

export VERSION:=$(shell ../bash/release_tag.sh)

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

build_command_wrapper_for_all_archs:
	rm -rf ./build

	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=arm64 \
	go build \
	  -o ./build/Linux/aarch64/command_wrapper \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	  ../infra/bots/task_drivers/command_wrapper

	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/command_wrapper \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	 ../infra/bots/task_drivers/command_wrapper

	CGO_ENABLED=0 \
	GOOS=darwin \
	GOARCH=amd64 \
	go build \
	 -o ./build/Darwin/x86_64/command_wrapper \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	 ../infra/bots/task_drivers/command_wrapper

	CGO_ENABLED=0 \
	GOOS=darwin \
	GOARCH=arm64 \
	go build \
	 -o ./build/Darwin/arm64/command_wrapper \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	 ../infra/bots/task_drivers/command_wrapper

	CGO_ENABLED=0 \
	GOOS=windows \
	GOARCH=amd64 \
	go build \
	 -o ./build/Win32NT/64-bit/command_wrapper \
	  -ldflags="-X 'main.Version=${VERSION}'" \
	 ../infra/bots/task_drivers/command_wrapper

build_and_upload_command_wrapper: build_command_wrapper_for_all_archs
	cipd create -pkg-def=command_wrapper_cipd.yml --tag version:${VERSION}

	#
	# The instance can be retrieved using the version tag:
	#
	#   cipd install --root=/tmp/sometempdir skia/internal/command_wrapper version:${VERSION}

# After running this, you may want to deploy install the tmm executable. To do so, see
# //skolo/ansible/switchboard/roles/install_test_machine_monitor/README.md
release_command_wrapper: build_and_upload_command_wrapper
	../bash/ansible-release.sh command_wrapper ${VERSION}


.PHONY: build generate serve testjs release push push_bazel \
    release_firestore_emulator start_firestore_emulator start_pubsub_emulator \
    build run-local-instance build \
    build_test_machine_monitor_for_all_archs \
    build_and_upload_test_machine_monitor \
    build_test_machine_monitor_for_all_archs release_test_machine_monitor \
    build_and_upload_test_machine_monitor
