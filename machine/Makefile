include ../make/bazel.mk

build: web
	go install -v ./go/...

web: ./modules/json/index.ts npm-ci
	npx webpack --mode=development

./modules/json/index.ts: ./go/machine/machine.go
	go generate ./go/...

watch: npm-ci
	npx webpack --mode=development --watch

serve: npm-ci
	npx webpack-dev-server --mode=development

test: npm-ci
	go test -cover ./go/...
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

testjs: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

debugjs: npm-ci
	npx karma start --no-single-run

.PHONY: puppeteer_tests
puppeteer_tests: npm-ci
	npx mocha --require ts-node/register ./**/*_puppeteer_test.ts

testci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

release_legacy: build npm-ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/machineserver
	./images/machineserver/release_legacy

push_legacy: release_legacy
	pushk machineserver

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

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

build-remote:
	$(BAZEL) build //machine/go/machineserver:machineserver --config=remote --remote_instance_name=projects/skia-public/instances/default_instance

run-local-instance:
	machineserver --local

export HASH=$(shell git rev-parse HEAD)
export DATETIME=$(shell date -u "+%Y-%m-%dT%H:%M:%SZ")
export GIT_STATE=$(shell ../bash/gitstate.sh)
export VERSION=${DATETIME}-${USER}-${HASH}-${GIT_STATE}

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

release_test_machine_monitor: build_and_upload_test_machine_monitor
	../bash/ansible-release.sh test_machine_monitor ${VERSION}

include ../make/npm.mk
