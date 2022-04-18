push:
	$(BAZEL) run //comp-ui:pushk_comp-ui-gitcron


export VERSION:=$(shell ../bash/release_tag.sh)

# This build must be launched from an Ansible playbook which will extract the
# service account keys and base64 encode them into the SERVICE_ACCOUNT_KEY
# environment variable.
#
# The directory under ./build/Linux/x86_64/ is the name of the secret where the
# service key is stored under.
build_compui_ansible:
	rm -rf ./build

	test -n "${SERVICE_ACCOUNT_KEY}" || (echo "SERVICE_ACCOUNT_KEY must be set by Ansible playbook."; exit 1)

	CGO_ENABLED=0 \
	GOOS=darwin \
	GOARCH=arm64 \
	go build \
	 -o ./build/Darwin/arm64/comp-ui-cron-job \
	 -ldflags="-X 'main.Version=${VERSION}' -X 'main.Key=${SERVICE_ACCOUNT_KEY}' " \
	 ./go/comp-ui-cron-job

build_and_upload_compui: build_compui_ansible
	cipd create -pkg-def=comp-ui-cron-job-cipd.yml --tag version:${VERSION}

release_compui: build_and_upload_compui
	../bash/ansible-release.sh comp-ui-cron-job ${VERSION}
