default: build npm-ci
	npx webpack --mode=development

build:
	go install ./go/hashtag

release: npm-ci
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/hashtag
	./build_release

push: release
	pushk hashtag

watch: npm-ci
	npx webpack --mode=development --watch

run: build
	# To run locally download the service account and point to it via the
	# GOOGLE_APPLICATION_CREDENTIALS environment variable.
	GOOGLE_APPLICATION_CREDENTIALS=${HOME}/.hashtag_creds hashtag --local --logtostderr

test:
	go test ./go/...

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

include ../make/npm.mk
