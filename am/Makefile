default: build npm-ci
	npx webpack --mode=development

./modules/json/index.ts:
	go generate ./go/types/ts

build: ./modules/json/index.ts
	go install ./go/alert-manager ./go/alert-to-pubsub

release: build npm-ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/alert-manager ./go/alert-to-pubsub
	./build_alert_manager_release
	./build_alert_to_pubsub_release

debug: npm-ci
	npx webpack --mode=development

legacy_release: build npm-ci
	./build_legacy_alert_to_pubsub_release "`git log -n1 --format=%s`"

push: release
	pushk alert-manager alert-to-pubsub

push_am: release
	pushk alert-manager

serve: npm-ci
	npx webpack-dev-server --mode=development --watch-poll

testgo:
	go test ./go/...

test: npm-ci
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: testpptr
testpptr: npm-ci
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha --require ts-node/register ./**/*_puppeteer_test.ts

testci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

watch: npm-ci
	npx webpack --mode=development --watch

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

export HASH:=$(shell git rev-parse HEAD)
export DATETIME:=$(shell date --utc "+%Y-%m-%dT%H:%M:%SZ")
export GIT_STATE:=$(shell ../bash/gitstate.sh)
export VERSION:=${DATETIME}-${USER}-${HASH}-${GIT_STATE}

build_alert_to_pubsub_ansible:
	rm -rf ./build
	CGO_ENABLED=0 \
	GOOS=linux \
	GOARCH=amd64 \
	go build \
	 -o ./build/Linux/x86_64/alert_to_pubsub_ansible \
	 -ldflags="-X 'main.Version=${VERSION}' " \
	 ./go/alert-to-pubsub

build_and_upload_alert_to_pubsub_ansible: build_alert_to_pubsub_ansible
	cipd create -pkg-def=alert_to_pubsub_ansible_cipd.yml --tag version:${VERSION}

	#
	# The instance can be retrieved using the version tag:
	#
	#   cipd install --root=/tmp/sometempdir skia/internal/alert_to_pubsub_ansible version:${VERSION}

release_alert_to_pubsub_ansible: build_and_upload_alert_to_pubsub_ansible
	../bash/ansible-release.sh alert_to_pubsub_ansible ${VERSION}

include ../make/npm.mk
