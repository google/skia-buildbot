build: web
	go install -v ./go/...

web: ./modules/json/index.ts npm-ci
	npx webpack --mode=development

./modules/json/index.ts: ./go/machine/machine.go
	go generate ./go/...

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
	npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

testci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

release: build npm-ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/machineserver
	./images/machineserver/release

watch: npm-ci
	npx webpack --mode=development --watch

push: release
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
	bazel build //machine/go/machineserver:machineserver --config=remote --remote_instance_name=projects/skia-public/instances/default_instance

run-local-instance:
	machineserver --local --logtostderr

include ../make/npm.mk
