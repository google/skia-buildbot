default: ./modules/json/machine.ts build package-lock.json
	npx webpack --mode=development

./modules/json/machine.ts: $GOPATH/bin/struct2ts ./go/machine/machine.go
	-mkdir -p modules/json/
	struct2ts --interface --no-helpers --out=./modules/json/machine.ts machine.Description

$GOPATH/bin/struct2ts:
	go get -u -v github.com/OneOfOne/struct2ts/...

build:
	go install -v ./go/...

test:
	go test -cover ./go/...
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

testjs:
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

debugjs:
	npx karma start --no-single-run

testci:
	rm -rf node_modules
	npm install
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

release: build package-lock.json
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/machineserver
	./images/machineserver/release

watch: package-lock.json
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

package-lock.json: package.json
	npm install

.PHONY: test-frontend-ci
test-frontend-ci:
	npm ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run
